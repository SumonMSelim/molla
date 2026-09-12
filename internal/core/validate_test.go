package core

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		wantErr bool
	}{
		{name: "http", rawURL: "http://example.com/path"},
		{name: "https", rawURL: "https://example.com/path?q=value#fragment"},
		{name: "case insensitive scheme", rawURL: "HTTPS://example.com"},
		{name: "javascript", rawURL: "javascript:alert(1)", wantErr: true},
		{name: "data", rawURL: "data:text/plain,hello", wantErr: true},
		{name: "file", rawURL: "file:///tmp/file", wantErr: true},
		{name: "ftp", rawURL: "ftp://example.com/file", wantErr: true},
		{name: "schemeless", rawURL: "example.com/path", wantErr: true},
		{name: "missing host", rawURL: "https:///path", wantErr: true},
		{name: "userinfo", rawURL: "https://user:pass@example.com", wantErr: true},
		{name: "control character", rawURL: "https://example.com/\nnext", wantErr: true},
		{name: "empty", rawURL: "", wantErr: true},
		{name: "invalid utf8", rawURL: "https://example.com/" + string([]byte{0xff}), wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateURL(test.rawURL)
			if test.wantErr && !errors.Is(err, ErrInvalidURL) {
				t.Fatalf("error = %v, want %v", err, ErrInvalidURL)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateURLCharacterLimit(t *testing.T) {
	const prefix = "https://example.com/"
	atLimit := prefix + strings.Repeat("a", MaxURLCharacters-len(prefix))
	overLimit := atLimit + "a"

	if count := utf8.RuneCountInString(atLimit); count != MaxURLCharacters {
		t.Fatalf("test URL length = %d", count)
	}
	if err := ValidateURL(atLimit); err != nil {
		t.Fatalf("2048-character URL: %v", err)
	}
	if err := ValidateURL(overLimit); !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("2049-character URL error = %v", err)
	}
}

func TestValidateAlias(t *testing.T) {
	tests := []struct {
		name    string
		alias   string
		wantErr bool
	}{
		{name: "minimum", alias: "a-1"},
		{name: "maximum", alias: strings.Repeat("Z", MaxAliasLength)},
		{name: "underscore", alias: "my_link"},
		{name: "too short", alias: "ab", wantErr: true},
		{name: "too long", alias: strings.Repeat("a", MaxAliasLength+1), wantErr: true},
		{name: "invalid character", alias: "my.link", wantErr: true},
		{name: "non ascii", alias: "mølla", wantErr: true},
		{name: "reserved api", alias: "api", wantErr: true},
		{name: "reserved app", alias: "app", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateAlias(test.alias)
			if test.wantErr && !errors.Is(err, ErrInvalidAlias) {
				t.Fatalf("error = %v, want %v", err, ErrInvalidAlias)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateExpiry(t *testing.T) {
	tests := []struct {
		seconds int64
		wantErr bool
	}{
		{MinExpirySeconds - 1, true},
		{MinExpirySeconds, false},
		{MaxExpirySeconds, false},
		{MaxExpirySeconds + 1, true},
	}

	for _, test := range tests {
		err := ValidateExpiry(test.seconds)
		if test.wantErr && !errors.Is(err, ErrInvalidExpiry) {
			t.Errorf("ValidateExpiry(%d) error = %v", test.seconds, err)
		}
		if !test.wantErr && err != nil {
			t.Errorf("ValidateExpiry(%d): %v", test.seconds, err)
		}
	}
}
