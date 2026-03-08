package hash

import "golang.org/x/crypto/bcrypt"

// DefaultCost is the bcrypt work factor used when hashing passwords.
// Cost 12 balances security and performance for a single-user admin system.
const DefaultCost = 12

// HashPassword returns the bcrypt hash of the given plain-text password.
func HashPassword(plain string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(plain), DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// CheckPassword compares a bcrypt-hashed password with its plain-text candidate.
// Returns nil on match, bcrypt.ErrMismatchedHashAndPassword on mismatch.
func CheckPassword(hashedPassword, plainPassword string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword))
}
