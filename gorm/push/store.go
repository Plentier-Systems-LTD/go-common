// Package push provides a GORM-backed device push-token store: register,
// unregister, and look up the tokens a user has registered, for use with
// go-common/push's Sender.
package push

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// deviceToken is one device's push token. A user may have several (phone +
// tablet, or a reinstalled app that got issued a new token) — Token is
// unique so re-registering the same device refreshes UpdatedAt instead of
// creating a duplicate row.
type deviceToken struct {
	ID        string    `gorm:"primaryKey"`
	UserID    string    `gorm:"index;not null"`
	Token     string    `gorm:"uniqueIndex;not null"`
	Platform  string    `gorm:"not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (deviceToken) TableName() string { return "device_tokens" }

// Store persists device push tokens.
type Store struct {
	db *gorm.DB
}

// NewStore wraps db as a device-token store, auto-migrating its own table.
func NewStore(db *gorm.DB) (*Store, error) {
	if err := db.AutoMigrate(&deviceToken{}); err != nil {
		return nil, fmt.Errorf("push: failed to auto-migrate schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Register upserts a device's push token for userID.
func (s *Store) Register(ctx context.Context, userID, token, platform string) error {
	rec := deviceToken{ID: uuid.NewString(), UserID: userID, Token: token, Platform: platform}
	err := s.db.WithContext(ctx).
		Where("token = ?", token).
		Assign(deviceToken{UserID: userID, Platform: platform}).
		FirstOrCreate(&rec).Error
	if err != nil {
		return fmt.Errorf("push: register token: %w", err)
	}
	return nil
}

// Unregister removes a device's token, e.g. when the user turns
// notifications off on that device.
func (s *Store) Unregister(ctx context.Context, userID, token string) error {
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND token = ?", userID, token).
		Delete(&deviceToken{}).Error
	if err != nil {
		return fmt.Errorf("push: unregister token: %w", err)
	}
	return nil
}

// TokensForUser returns every device token registered for userID.
func (s *Store) TokensForUser(ctx context.Context, userID string) ([]string, error) {
	var records []deviceToken
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("push: load tokens: %w", err)
	}
	tokens := make([]string, len(records))
	for i, r := range records {
		tokens[i] = r.Token
	}
	return tokens, nil
}
