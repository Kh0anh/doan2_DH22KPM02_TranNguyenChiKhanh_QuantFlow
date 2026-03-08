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
