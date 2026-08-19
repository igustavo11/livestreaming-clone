package auth

import (
	"errors"
	"regexp"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9_]{3,25}$`)

func ValidateUsername(username string) error {
	if !usernamePattern.MatchString(username) {
		return errors.New("username must be 3-25 chars, lowercase letters, digits and underscores only")
	}
	return nil
}
