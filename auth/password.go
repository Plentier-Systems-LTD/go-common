package auth

import "golang.org/x/crypto/bcrypt"

// PasswordHasher hashes and verifies passwords. Service depends on this
// interface rather than bcrypt directly, so a project can swap in a
// different algorithm (e.g. argon2) via WithPasswordHasher without
// touching Service.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

type bcryptHasher struct {
	cost int
}

// NewBcryptHasher returns the default PasswordHasher, backed by bcrypt.
// cost defaults to bcrypt.DefaultCost when omitted.
func NewBcryptHasher(cost ...int) PasswordHasher {
	c := bcrypt.DefaultCost
	if len(cost) > 0 && cost[0] > 0 {
		c = cost[0]
	}
	return bcryptHasher{cost: c}
}

func (b bcryptHasher) Hash(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), b.cost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func (b bcryptHasher) Compare(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
