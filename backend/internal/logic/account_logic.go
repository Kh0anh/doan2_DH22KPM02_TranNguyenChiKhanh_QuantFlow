package logic

import (
	"context"
	"errors"
	"fmt"

	"github.com/kh0anh/quantflow/internal/repository"
	"github.com/kh0anh/quantflow/pkg/hash"
)

// ErrInvalidCurrentPassword is returned when the supplied current_password does
// not match the BCrypt hash stored in the database.
// The HTTP handler maps this to 401 INVALID_CURRENT_PASSWORD (api.yaml §/account/profile).
var ErrInvalidCurrentPassword = errors.New("current password is incorrect")

// UpdateProfileInput is the internal DTO passed from AccountHandler to AccountLogic.
// At least one of NewUsername / NewPassword must be non-empty (validated by the handler).
type UpdateProfileInput struct {
	// UserID is the primary key UUID sourced from the verified JWT Claims.
	UserID string
	// CurrentPassword is the plain-text password the user claims is current.
	// BCrypt comparison is performed against the stored hash.
	CurrentPassword string
	// NewUsername is the replacement username. Empty string means "no change".
	NewUsername string
	// NewPassword is the replacement plain-text password. Empty string means "no change".
	NewPassword string
}

// AccountLogic orchestrates account-management business rules (WBS 2.1.6).
type AccountLogic struct {
	userRepo repository.UserRepository
}

// NewAccountLogic constructs an AccountLogic with its required dependencies.
func NewAccountLogic(userRepo repository.UserRepository) *AccountLogic {
	return &AccountLogic{userRepo: userRepo}
}

// UpdateProfile verifies the current password and applies the requested changes
// to the user record.
//
// Return patterns:
//   - nil                              — success; caller must perform Force Logout.
//   - ErrInvalidCurrentPassword        — current_password mismatch → HTTP 401.
//   - any other error                  — unexpected server-side error → HTTP 500.
//
// SRS FR-ACCESS-02 · UC-02 Business Rule 1 & 2 · NFR-SEC-02
func (l *AccountLogic) UpdateProfile(ctx context.Context, input UpdateProfileInput) error {
	// 1. Load the current user record to obtain the stored PasswordHash.
	user, err := l.userRepo.FindByID(ctx, input.UserID)
	if err != nil {
		return fmt.Errorf("account_logic: UpdateProfile: %w", err)
	}
	if user == nil {
		// Should never happen for an authenticated request, but guard defensively.
		return fmt.Errorf("account_logic: UpdateProfile: user not found for id %s", input.UserID)
	}

	// 2. Verify current_password via constant-time BCrypt comparison (NFR-SEC-02).
	if err := hash.CheckPassword(user.PasswordHash, input.CurrentPassword); err != nil {
		return ErrInvalidCurrentPassword
	}

	// 3. Apply changes — only mutate the fields the caller requested.
	if input.NewUsername != "" {
		user.Username = input.NewUsername
	}
	if input.NewPassword != "" {
		newHash, err := hash.HashPassword(input.NewPassword)
		if err != nil {
			return fmt.Errorf("account_logic: UpdateProfile: hash new password: %w", err)
		}
		user.PasswordHash = newHash
	}

	// 4. Persist the updated record (GORM db.Save sets updated_at automatically).
	if err := l.userRepo.UpdateUser(ctx, user); err != nil {
		return fmt.Errorf("account_logic: UpdateProfile: %w", err)
	}

	return nil
}
