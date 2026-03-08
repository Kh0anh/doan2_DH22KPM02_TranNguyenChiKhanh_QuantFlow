package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/kh0anh/quantflow/internal/domain"
	"gorm.io/gorm"
)

// UserRepository defines the data-access contract for the users table.
// Only the methods required by implemented tasks are declared here;
// additional methods will be added as subsequent tasks are completed.
type UserRepository interface {
	FindByUsername(ctx context.Context, username string) (*domain.User, error)
	// FindByID retrieves a user by primary key UUID.
	// WBS 2.1.6: needed to load PasswordHash for current_password verification.
	FindByID(ctx context.Context, id string) (*domain.User, error)
	// UpdateUser persists changes to an existing user record (username / password_hash).
	// GORM db.Save() updates all fields and sets updated_at automatically.
	UpdateUser(ctx context.Context, user *domain.User) error
}

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository constructs a GORM-backed UserRepository.
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// FindByUsername retrieves a user record by its unique username.
// Returns nil, nil when the user is not found (callers must handle this case).
func (r *userRepository) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).
		Where("username = ?", username).
		First(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("user_repo: FindByUsername: %w", err)
	}
	return &user, nil
}

// FindByID retrieves a user record by its primary key UUID.
// Returns nil, nil when the user is not found.
// WBS 2.1.6: used by AccountLogic to load PasswordHash for current_password verification.
func (r *userRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("user_repo: FindByID: %w", err)
	}
	return &user, nil
}

// UpdateUser persists all field changes for an existing user record.
// GORM db.Save() performs a full UPDATE, setting updated_at automatically
// via the autoUpdateTime tag on domain.User.
func (r *userRepository) UpdateUser(ctx context.Context, user *domain.User) error {
	if err := r.db.WithContext(ctx).Save(user).Error; err != nil {
		return fmt.Errorf("user_repo: UpdateUser: %w", err)
	}
	return nil
}
