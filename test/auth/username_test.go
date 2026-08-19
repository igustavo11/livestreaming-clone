package auth_test

import (
	"testing"

	. "github.com/igustavo11/livestreaming-clone/internal/auth"
)

func TestValidateUsername(t *testing.T) {
	valid := []string{
		"guguinha",
		"abc",
		"a_b_c",
		"user123",
		"abcdefghijklmnopqrstuvwxy", // 25 chars
	}
	for _, u := range valid {
		if err := ValidateUsername(u); err != nil {
			t.Errorf("ValidateUsername(%q) = %v, want nil", u, err)
		}
	}

	invalid := []string{
		"",                            // empty
		"ab",                          // too short
		"abcdefghijklmnopqrstuvwxyz",  // 26 chars, too long
		"Guguinha",                    // uppercase
		"gugu inha",                   // space
		"gugu-inha",                   // hyphen
		"gugu.inha",                   // dot
		"gugu@inha",                   // symbol
	}
	for _, u := range invalid {
		if err := ValidateUsername(u); err == nil {
			t.Errorf("ValidateUsername(%q) = nil, want error", u)
		}
	}
}
