package core

import (
	"errors"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	MaxURLCharacters     = 2048
	MinAliasLength       = 3
	MaxAliasLength       = 32
	MinExpirySeconds     = int64(60)
	DefaultExpirySeconds = int64(157680000)
	MaxExpirySeconds     = DefaultExpirySeconds
)

var (
	ErrInvalidURL    = errors.New("INVALID_URL")
	ErrInvalidAlias  = errors.New("INVALID_ALIAS")
	ErrInvalidExpiry = errors.New("INVALID_EXPIRY")
)

// ValidateURL checks a redirect target without making a network request.
func ValidateURL(rawURL string) error {
	if rawURL == "" || !utf8.ValidString(rawURL) ||
		utf8.RuneCountInString(rawURL) > MaxURLCharacters {
		return ErrInvalidURL
	}
	for _, char := range rawURL {
		if unicode.IsControl(char) {
			return ErrInvalidURL
		}
	}

	parsed, err := url.Parse(rawURL)
	if err != nil ||
		(!strings.EqualFold(parsed.Scheme, "http") &&
			!strings.EqualFold(parsed.Scheme, "https")) ||
		parsed.Host == "" ||
		parsed.User != nil {
		return ErrInvalidURL
	}
	return nil
}

// ValidateAlias checks a user-selected short code.
func ValidateAlias(alias string) error {
	if len(alias) < MinAliasLength || len(alias) > MaxAliasLength ||
		alias == "api" || alias == "app" {
		return ErrInvalidAlias
	}
	for i := 0; i < len(alias); i++ {
		char := alias[i]
		if (char < 'a' || char > 'z') &&
			(char < 'A' || char > 'Z') &&
			(char < '0' || char > '9') &&
			char != '-' && char != '_' {
			return ErrInvalidAlias
		}
	}
	return nil
}

// ValidateExpiry checks a requested link lifetime in seconds.
func ValidateExpiry(seconds int64) error {
	if seconds < MinExpirySeconds || seconds > MaxExpirySeconds {
		return ErrInvalidExpiry
	}
	return nil
}
