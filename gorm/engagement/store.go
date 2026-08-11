// Package engagement is a ready-made GORM implementation of
// go-common/engagement.Store, so most projects never need to write their
// own persistence layer to use engagement.Recorder.
package engagement

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	sharedengagement "github.com/Plentier-Systems-LTD/go-common/engagement"
)

// eventRecord is the GORM persistence model — never exposed outside this
// package; callers only ever see sharedengagement.Event/EventType.
type eventRecord struct {
	ID        string                     `gorm:"primaryKey"`
	UserID    string                     `gorm:"index;not null"`
	Type      sharedengagement.EventType `gorm:"index;not null"`
	CreatedAt time.Time                  `gorm:"autoCreateTime;index"`
}

func (eventRecord) TableName() string { return "engagement_events" }

// Store is a sharedengagement.Store backed by GORM.
type Store struct {
	db *gorm.DB
}

// NewStore wraps db as a sharedengagement.Store, auto-migrating its own
// table — a platform never writes a migration for this itself.
func NewStore(db *gorm.DB) (*Store, error) {
	if err := db.AutoMigrate(&eventRecord{}); err != nil {
		return nil, fmt.Errorf("engagement: failed to migrate: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Record(ctx context.Context, event sharedengagement.Event) error {
	at := event.At
	if at.IsZero() {
		at = time.Now()
	}
	return s.db.WithContext(ctx).Create(&eventRecord{
		ID:        uuid.NewString(),
		UserID:    event.UserID,
		Type:      event.Type,
		CreatedAt: at,
	}).Error
}

func (s *Store) Total(ctx context.Context, eventType sharedengagement.EventType) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&eventRecord{}).Where("type = ?", eventType).Count(&count).Error
	return count, err
}

func (s *Store) DailyCounts(ctx context.Context, eventType sharedengagement.EventType, since time.Time) (map[string]int64, error) {
	var rows []struct {
		Day   time.Time
		Count int64
	}
	err := s.db.WithContext(ctx).Model(&eventRecord{}).
		Select("date_trunc('day', created_at) AS day, count(*) AS count").
		Where("type = ? AND created_at >= ?", eventType, since).
		Group("day").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	byDay := make(map[string]int64, len(rows))
	for _, r := range rows {
		byDay[r.Day.Format("2006-01-02")] = r.Count
	}
	return byDay, nil
}
