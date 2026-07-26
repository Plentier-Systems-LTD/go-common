package auth

import (
	"context"
	"sync"
)

// AsyncEmailSenderConfig tunes AsyncEmailSender.
type AsyncEmailSenderConfig struct {
	// QueueSize is how many pending sends can be buffered before
	// SendVerificationCode blocks the caller. Defaults to 64.
	QueueSize int

	// Workers is how many goroutines send queued emails concurrently.
	// Defaults to 4.
	Workers int

	// OnError is called from a worker goroutine when the wrapped
	// EmailSender fails to deliver. Defaults to a no-op. Once a send is
	// queued its caller can no longer see the outcome, so wire this up to
	// your logger/error tracker if you need to know about failures.
	OnError func(to string, err error)
}

func (c AsyncEmailSenderConfig) withDefaults() AsyncEmailSenderConfig {
	if c.QueueSize <= 0 {
		c.QueueSize = 64
	}
	if c.Workers <= 0 {
		c.Workers = 4
	}
	if c.OnError == nil {
		c.OnError = func(string, error) {}
	}
	return c
}

type emailJob struct {
	to   string
	code string
}

// AsyncEmailSender wraps an EmailSender so SendVerificationCode enqueues
// the send and returns immediately, instead of blocking its caller (e.g.
// an HTTP handler, via Service.SendVerificationEmail) on the round trip
// to your email provider. It's a decorator — Service never knows or
// cares whether the EmailSender it was given is synchronous or not.
type AsyncEmailSender struct {
	next    EmailSender
	jobs    chan emailJob
	onError func(to string, err error)

	wg        sync.WaitGroup
	closeOnce sync.Once
}

var _ EmailSender = (*AsyncEmailSender)(nil)

// NewAsyncEmailSender wraps sender so it delivers emails in the
// background instead of on the caller's goroutine.
func NewAsyncEmailSender(sender EmailSender, cfg ...AsyncEmailSenderConfig) *AsyncEmailSender {
	c := AsyncEmailSenderConfig{}
	if len(cfg) > 0 {
		c = cfg[0]
	}
	c = c.withDefaults()

	a := &AsyncEmailSender{
		next:    sender,
		jobs:    make(chan emailJob, c.QueueSize),
		onError: c.OnError,
	}

	a.wg.Add(c.Workers)
	for i := 0; i < c.Workers; i++ {
		go a.worker()
	}

	return a
}

func (a *AsyncEmailSender) worker() {
	defer a.wg.Done()
	for job := range a.jobs {
		// A fresh, uncancelable context: the request that triggered this
		// send has likely already finished by the time this runs, so its
		// context's deadline/cancellation no longer applies.
		if err := a.next.SendVerificationCode(context.Background(), job.to, job.code); err != nil {
			a.onError(job.to, err)
		}
	}
}

// SendVerificationCode enqueues the email and returns as soon as it's
// queued — it does not wait for delivery. Errors from the wrapped
// EmailSender are reported to AsyncEmailSenderConfig.OnError, not
// returned here. If the queue is full, this blocks (providing
// backpressure) until space frees up or ctx is canceled.
func (a *AsyncEmailSender) SendVerificationCode(ctx context.Context, to string, code string) error {
	select {
	case a.jobs <- emailJob{to: to, code: code}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close stops accepting new sends and blocks until every already-queued
// email has been processed. Call it during graceful shutdown so
// in-flight emails aren't dropped when the process exits.
func (a *AsyncEmailSender) Close() {
	a.closeOnce.Do(func() { close(a.jobs) })
	a.wg.Wait()
}
