package domain

import "time"

// User maps to the `users` table in PostgreSQL.
// It is the aggregate root for authentication and account management.
type User struct {
	ID           string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Username     string    `gorm:"type:varchar(50);uniqueIndex;not null"          json:"username"`
	PasswordHash string    `gorm:"type:varchar(255);not null"                     json:"-"`
	CreatedAt    time.Time `gorm:"not null;autoCreateTime"                        json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"                                 json:"updated_at"`
}

// TableName overrides the default GORM table name.
func (User) TableName() string { return "users" }

// UserBasic is a read-only response DTO returned by /auth/login and /auth/me.
// It deliberately excludes PasswordHash and UpdatedAt.
type UserBasic struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

// ToBasic projects a User entity into its public-safe representation.
func (u *User) ToBasic() UserBasic {
	return UserBasic{
		ID:        u.ID,
		Username:  u.Username,
		CreatedAt: u.CreatedAt,
	}
}
