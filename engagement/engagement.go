// Package engagement records aggregate, non-PII product-usage events —
// "this kind of thing happened, for this user, at this time" — the moment
// it actually happens, so a platform-stats dashboard can chart real usage
// (files uploaded, AI calls made) instead of inferring it after the fact
// by counting rows in a platform's own domain tables (which, in practice,
// drifts: different apps name and shape that data differently, and some
// of it is awkward or impossible to query from outside the app's own
// models package). Deliberately minimal: no event payloads, no content,
// nothing that could reconstruct what a user actually did or said — only
// that an event of a given type occurred.
//
// Framework/storage-agnostic; see gorm/engagement for a ready-made GORM
// Store.
package engagement

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// EventType names one kind of engagement event. Free-form — go-common
// doesn't enumerate a platform's features — but prefer the Standard
// constants below where they fit: the platform-stats dashboard already
// knows how to chart exactly these three (they map straight onto
// adminapi.PlatformStats's FilesUploadedTotal/AIAnalysesTotal/
// AIChatMessagesTotal and the matching TrendPoint fields), so using them
// means zero dashboard-side changes to show up.
type EventType string

const (
	// FileUploaded is a user successfully uploading/attaching a file — a
	// document, a photo, anything that lands in real storage. Record it
	// right after the upload succeeds, not when the request merely
	// starts.
	FileUploaded EventType = "file_uploaded"
	// AIAnalysis is one AI call that analyzes something already
	// uploaded (a document, a photo) and produces a structured result —
	// record it once per call, regardless of whether the result was
	// good, so it reflects "AI was invoked" not "AI succeeded."
	AIAnalysis EventType = "ai_analysis"
	// AIChatMessage is one AI-generated reply in a conversation. Record
	// it once per assistant reply, not per user message.
	AIChatMessage EventType = "ai_chat_message"
)

// Event is one recorded engagement action.
type Event struct {
	UserID string
	Type   EventType
	// At is when the event happened; a zero value means "now" — Store
	// implementations should treat it that way rather than persisting a
	// zero timestamp.
	At time.Time
}

// Store persists and queries engagement events. Implement it against
// whatever database a project uses — see gorm/engagement.NewStore for a
// ready-made GORM implementation that auto-migrates its own table, so a
// platform never writes its own migration for this.
type Store interface {
	Record(ctx context.Context, event Event) error
	// Total counts every event of type ever recorded — an all-time
	// total, mirroring adminapi.PlatformStats's *Total fields.
	Total(ctx context.Context, eventType EventType) (int64, error)
	// DailyCounts groups events of type recorded since `since` by
	// calendar day (UTC), keyed by "2006-01-02" — the same shape
	// adminapi.TrendPoint's per-day series expects.
	DailyCounts(ctx context.Context, eventType EventType, since time.Time) (map[string]int64, error)
}

// RecorderConfig tunes Recorder. All fields default to sane values when
// left zero.
type RecorderConfig struct {
	// QueueSize is how many pending events can be buffered before Record
	// starts dropping them. Defaults to 256.
	QueueSize int
	// Workers is how many goroutines write queued events concurrently.
	// Defaults to 2 — this is lightweight, infrequent work, not a
	// high-throughput pipeline.
	Workers int
	// OnError is called from a worker goroutine when the Store fails to
	// persist an event, or from the caller's own goroutine when the
	// queue is full and an event is dropped. Defaults to a no-op; wire
	// it to your logger if you want to know about either.
	OnError func(err error)
}

func (c RecorderConfig) withDefaults() RecorderConfig {
	if c.QueueSize <= 0 {
		c.QueueSize = 256
	}
	if c.Workers <= 0 {
		c.Workers = 2
	}
	if c.OnError == nil {
		c.OnError = func(error) {}
	}
	return c
}

// Recorder is the "plug and play" call site: wrap a Store once at
// startup, then call Record at the exact moment a real action happens —
// it never blocks the caller and never fails the request it's attached
// to. Losing an occasional event under a queue-full spike is an
// acceptable tradeoff for a usage-stats feature; blocking or failing a
// real user request to guarantee delivery is not. Same async-queue shape
// as auth.AsyncEmailSender, for the same reason.
type Recorder struct {
	store Store
	queue chan Event
	onErr func(error)
	wg    sync.WaitGroup
}

// NewRecorder builds a Recorder around store and starts its worker pool.
// Call Close during graceful shutdown to flush queued events.
func NewRecorder(store Store, cfg RecorderConfig) *Recorder {
	cfg = cfg.withDefaults()
	r := &Recorder{
		store: store,
		queue: make(chan Event, cfg.QueueSize),
		onErr: cfg.OnError,
	}
	for i := 0; i < cfg.Workers; i++ {
		r.wg.Add(1)
		go r.worker()
	}
	return r
}

func (r *Recorder) worker() {
	defer r.wg.Done()
	for event := range r.queue {
		if err := r.store.Record(context.Background(), event); err != nil {
			r.onErr(fmt.Errorf("engagement: failed to record %s event for user %s: %w", event.Type, event.UserID, err))
		}
	}
}

// Record enqueues one event for userID — call it right after the real
// action succeeds (a file lands in storage, an AI call returns), not
// before. Returns immediately; never blocks, never returns an error to
// the caller.
func (r *Recorder) Record(userID string, eventType EventType) {
	event := Event{UserID: userID, Type: eventType, At: time.Now()}
	select {
	case r.queue <- event:
	default:
		r.onErr(fmt.Errorf("engagement: queue full, dropped %s event for user %s", eventType, userID))
	}
}

// Close stops accepting new events and waits for every already-queued
// event to be written. Safe to call once during graceful shutdown.
func (r *Recorder) Close() {
	close(r.queue)
	r.wg.Wait()
}
