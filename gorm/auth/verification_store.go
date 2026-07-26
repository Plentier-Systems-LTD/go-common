package auth

import (
	"context"
	"errors"

	sharedauth "github.com/Plentier-Systems-LTD/go-common/auth"
	"gorm.io/gorm"
)

// VerificationStore is a sharedauth.VerificationStore backed by GORM.
// Unlike Store[T], its schema is fixed and owned entirely by this
// package, so NewVerificationStore migrates its own table for you.
type VerificationStore struct {
	db *gorm.DB
}

// NewVerificationStore wraps a *gorm.DB as a sharedauth.VerificationStore.
func NewVerificationStore(db *gorm.DB) (*VerificationStore, error) {
	if err := db.AutoMigrate(&sharedauth.VerificationCode{}); err != nil {
		return nil, err
	}
	return &VerificationStore{db: db}, nil
}

func (s *VerificationStore) Create(ctx context.Context, code sharedauth.VerificationCode) error {
	return s.db.WithContext(ctx).Create(&code).Error
}

func (s *VerificationStore) FindLatest(ctx context.Context, email string) (sharedauth.VerificationCode, error) {
	var code sharedauth.VerificationCode
	err := s.db.WithContext(ctx).
		Where("email = ?", email).
		Order("created_at DESC").
		First(&code).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sharedauth.VerificationCode{}, sharedauth.ErrVerificationCodeNotFound
		}
		return sharedauth.VerificationCode{}, err
	}
	return code, nil
}

func (s *VerificationStore) IncrementAttempts(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).
		Model(&sharedauth.VerificationCode{}).
		Where("id = ?", id).
		UpdateColumn("attempts", gorm.Expr("attempts + 1")).Error
}

func (s *VerificationStore) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&sharedauth.VerificationCode{}, "id = ?", id).Error
}
