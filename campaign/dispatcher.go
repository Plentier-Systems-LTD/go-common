package campaign

import (
	"context"
	"time"
)

// Dispatcher owns the ticker goroutine that sends scheduled campaigns once they come due.
type Dispatcher struct {
	svc      *Service
	interval time.Duration
	onError  func(campaignID uint, err error)
	stop     chan struct{}
}

// NewDispatcher builds a Dispatcher. onError, if non-nil, is called whenever a due campaign fails to send.
func NewDispatcher(svc *Service, interval time.Duration, onError func(campaignID uint, err error)) *Dispatcher {
	if interval <= 0 {
		interval = time.Minute
	}
	return &Dispatcher{svc: svc, interval: interval, onError: onError, stop: make(chan struct{})}
}

// Start checks for due campaigns once immediately, then again every interval, until Stop is called.
func (d *Dispatcher) Start() {
	go d.run()
}

// Stop ends the dispatch loop. Safe to call once during graceful shutdown.
func (d *Dispatcher) Stop() {
	close(d.stop)
}

func (d *Dispatcher) run() {
	_ = d.DispatchDue(context.Background())

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = d.DispatchDue(context.Background())
		case <-d.stop:
			return
		}
	}
}

// DispatchDue sends every scheduled campaign whose ScheduledAt has passed.
func (d *Dispatcher) DispatchDue(ctx context.Context) error {
	var due []Campaign
	now := time.Now()
	err := d.svc.db.WithContext(ctx).
		Where("status = ? AND scheduled_at <= ?", StatusScheduled, now).
		Find(&due).Error
	if err != nil {
		if d.onError != nil {
			d.onError(0, err)
		}
		return err
	}

	for _, c := range due {
		if _, sendErr := d.svc.SendNow(ctx, c.ID); sendErr != nil && d.onError != nil {
			d.onError(c.ID, sendErr)
		}
	}
	return nil
}
