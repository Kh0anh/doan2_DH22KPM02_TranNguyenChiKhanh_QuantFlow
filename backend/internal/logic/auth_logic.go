package logic

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/kh0anh/quantflow/internal/domain"
	"github.com/kh0anh/quantflow/internal/repository"
	"github.com/kh0anh/quantflow/pkg/hash"
)

// Brute-force protection constants (FR-ACCESS-01, UC-01 Business Rule 1).
const (
	MaxLoginAttempts = 5
	LockoutDuration  = 15 * time.Minute
)

// Sentinel errors used by callers to distinguish failure modes.
var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrAccountLocked      = errors.New("account temporarily locked")
)

// LockInfo carries the context returned alongside ErrInvalidCredentials or
// ErrAccountLocked so that the HTTP handler can build the exact API response
// described in api.yaml.
type LockInfo struct {
	// RemainingAttempts is the number of tries left before the account is locked.
	// Populated only when ErrInvalidCredentials is returned.
	RemainingAttempts int

	// LockedUntil is the UTC timestamp when the lockout expires.
	// Populated only when ErrAccountLocked is returned.
	LockedUntil time.Time
}

// loginAttempt tracks the running failure count and optional lockout for a
// single username. All mutations are guarded by BruteForceStore.mu.
type loginAttempt struct {
	count       int
	lockedUntil time.Time
}

// BruteForceStore is a thread-safe in-memory store for login attempt tracking.
// Per tech_stack.md Layer 4: brute-force protection is intentionally in-memory
// (no DB round-trip) because the system uses a single-user model.
type BruteForceStore struct {
	mu       sync.Mutex
	attempts map[string]*loginAttempt
}

// NewBruteForceStore constructs an empty BruteForceStore.
func NewBruteForceStore() *BruteForceStore {
	return &BruteForceStore{attempts: make(map[string]*loginAttempt)}
}

// Check returns (isLocked, lockInfo).
// If the username is currently locked, isLocked = true and lockInfo.LockedUntil
// is set. If the lock has expired it is automatically cleared.
func (s *BruteForceStore) Check(username string) (bool, LockInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	a, ok := s.attempts[username]
	if !ok {
		return false, LockInfo{}
	}

	if !a.lockedUntil.IsZero() {
		if time.Now().UTC().Before(a.lockedUntil) {
			return true, LockInfo{LockedUntil: a.lockedUntil}
		}
		// Lock has expired — reset so the next attempt starts fresh.
		delete(s.attempts, username)
	}
	return false, LockInfo{}
}

// Increment adds one failure for username and locks the account when
// MaxLoginAttempts is reached. Returns the updated LockInfo.
func (s *BruteForceStore) Increment(username string) LockInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	a, ok := s.attempts[username]
	if !ok {
		a = &loginAttempt{}
		s.attempts[username] = a
	}
	a.count++

	if a.count >= MaxLoginAttempts {
		a.lockedUntil = time.Now().UTC().Add(LockoutDuration)
		return LockInfo{LockedUntil: a.lockedUntil}
	}
	return LockInfo{RemainingAttempts: MaxLoginAttempts - a.count}
}

// Reset clears all failure records for username after a successful login.
func (s *BruteForceStore) Reset(username string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.attempts, username)
}

// AuthLogic orchestrates authentication business rules.
type AuthLogic struct {
	userRepo   repository.UserRepository
	bruteForce *BruteForceStore
}

// NewAuthLogic constructs an AuthLogic with its required dependencies.
func NewAuthLogic(userRepo repository.UserRepository, bruteForce *BruteForceStore) *AuthLogic {
	return &AuthLogic{
		userRepo:   userRepo,
		bruteForce: bruteForce,
	}
}

// Login validates credentials and enforces brute-force protection.
//
// Return patterns:
//   - (*User, nil, nil)             — success; caller should issue JWT.
//   - (nil, *LockInfo, ErrAccountLocked)      — account is locked.
//   - (nil, *LockInfo, ErrInvalidCredentials) — wrong credentials; includes remaining attempts.
//   - (nil, nil, err)               — unexpected server-side error.
func (l *AuthLogic) Login(ctx context.Context, username, password string) (*domain.User, *LockInfo, error) {
	// 1. Check brute-force lock before touching the database.
	if locked, info := l.bruteForce.Check(username); locked {
		return nil, &info, ErrAccountLocked
	}

	// 2. Look up the user record.
	user, err := l.userRepo.FindByUsername(ctx, username)
	if err != nil {
		return nil, nil, err
	}
	if user == nil {
		// Username not found — increment counter to prevent username enumeration
		// being trivially distinguished from wrong-password by response timing.
		info := l.bruteForce.Increment(username)
		return nil, &info, ErrInvalidCredentials
	}

	// 3. Verify password hash (constant-time comparison via bcrypt).
	if err := hash.CheckPassword(user.PasswordHash, password); err != nil {
		info := l.bruteForce.Increment(username)
		return nil, &info, ErrInvalidCredentials
	}

	// 4. Successful login — clear any stale failure records.
	l.bruteForce.Reset(username)
	return user, nil, nil
}
