package auth

import (
	"context"
	"testing"
)

// testUser is a minimal User for exercising Service without a real database.
type testUser struct {
	BaseUser
}

// memStore is an in-memory UserStore[*testUser] for tests.
type memStore struct {
	byID map[string]*testUser
}

func newMemStore() *memStore {
	return &memStore{byID: map[string]*testUser{}}
}

func (s *memStore) Create(_ context.Context, user *testUser) error {
	s.byID[user.ID] = user
	return nil
}

func (s *memStore) Update(_ context.Context, user *testUser) error {
	s.byID[user.ID] = user
	return nil
}

func (s *memStore) FindByID(_ context.Context, id string) (*testUser, error) {
	if u, ok := s.byID[id]; ok {
		return u, nil
	}
	return nil, ErrUserNotFound
}

func (s *memStore) FindByEmail(_ context.Context, email string) (*testUser, error) {
	for _, u := range s.byID {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, ErrUserNotFound
}

func (s *memStore) FindByProvider(_ context.Context, provider Provider, providerID string) (*testUser, error) {
	for _, u := range s.byID {
		if u.Provider == provider && u.ProviderID == providerID {
			return u, nil
		}
	}
	return nil, ErrUserNotFound
}

func (s *memStore) FindByGuestKey(_ context.Context, guestKey string) (*testUser, error) {
	for _, u := range s.byID {
		if u.GuestKey != nil && *u.GuestKey == guestKey {
			return u, nil
		}
	}
	return nil, ErrUserNotFound
}

func newUser() *testUser { return &testUser{} }

// TestContinueAsGuestReusesSameAccount is the guarantee this whole feature exists for: repeated guest sign-ins on one device must resolve to one account, not a new one each time.
func TestContinueAsGuestReusesSameAccount(t *testing.T) {
	svc, err := NewService[*testUser](newMemStore(), Config{Secret: "test-secret"})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	ctx := context.Background()

	first, _, isNew, err := svc.ContinueAsGuest(ctx, "device-abc", newUser)
	if err != nil {
		t.Fatalf("first ContinueAsGuest: %v", err)
	}
	if !isNew {
		t.Fatal("first ContinueAsGuest: expected isNew=true")
	}

	second, _, isNew, err := svc.ContinueAsGuest(ctx, "device-abc", newUser)
	if err != nil {
		t.Fatalf("second ContinueAsGuest: %v", err)
	}
	if isNew {
		t.Fatal("second ContinueAsGuest: expected isNew=false, got a fresh account")
	}
	if second.GetID() != first.GetID() {
		t.Fatalf("second ContinueAsGuest: got a different account (%s) than the first (%s)", second.GetID(), first.GetID())
	}
	if second.GetEmail() != first.GetEmail() {
		t.Fatalf("second ContinueAsGuest: email changed between calls (%s -> %s)", first.GetEmail(), second.GetEmail())
	}

	other, _, isNew, err := svc.ContinueAsGuest(ctx, "device-xyz", newUser)
	if err != nil {
		t.Fatalf("third ContinueAsGuest: %v", err)
	}
	if !isNew {
		t.Fatal("third ContinueAsGuest: a different guestKey should create a different account")
	}
	if other.GetID() == first.GetID() {
		t.Fatal("third ContinueAsGuest: got the first device's account for a different guestKey")
	}
}

// TestContinueAsGuestRequiresGuestKey guards against a client bug silently reusing one shared account.
func TestContinueAsGuestRequiresGuestKey(t *testing.T) {
	svc, err := NewService[*testUser](newMemStore(), Config{Secret: "test-secret"})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if _, _, _, err := svc.ContinueAsGuest(context.Background(), "", newUser); err == nil {
		t.Fatal("expected an error for an empty guestKey")
	}
}
