package world

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

func hashAccountPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func verifyAccountPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func isBcryptHash(value string) bool {
	_, err := bcrypt.Cost([]byte(value))
	return err == nil
}
