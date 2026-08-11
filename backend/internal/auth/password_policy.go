package auth

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/browser-gateway/backend/internal/domain"
)

func (s *Service) ValidatePassword(password string) error {
	var settings domain.AppSettings
	minLen := 8
	requireComplexity := false
	if err := s.db.First(&settings).Error; err == nil {
		if settings.PasswordMinLength > 0 {
			minLen = settings.PasswordMinLength
		}
		requireComplexity = settings.PasswordRequireComplexity
	}
	if len(password) < minLen {
		return fmt.Errorf("%w: minimum %d characters", ErrWeakPassword, minLen)
	}
	if requireComplexity {
		var upper, lower, digit, special bool
		for _, r := range password {
			switch {
			case unicode.IsUpper(r):
				upper = true
			case unicode.IsLower(r):
				lower = true
			case unicode.IsDigit(r):
				digit = true
			case unicode.IsPunct(r) || unicode.IsSymbol(r):
				special = true
			}
		}
		if !upper || !lower || !digit || !special {
			return fmt.Errorf("%w: need upper, lower, digit, and symbol", ErrWeakPassword)
		}
	}
	_ = strings.TrimSpace(password)
	return nil
}
