// Package core holds the provider-neutral domain logic: Base62 encoding,
// the keyed permutation that hides counter order, URL and alias validation,
// and the redirect decision. It imports nothing outside the standard library.
package core

import "errors"

const (
	// CodeLength is the fixed length of generated short codes.
	CodeLength = 7
	// Base62Limit is the number of distinct seven-character Base62 codes.
	Base62Limit uint64 = 62 * 62 * 62 * 62 * 62 * 62 * 62
)

const base62Alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var ErrInvalidBase62 = errors.New("invalid Base62 value")

// EncodeBase62 encodes an allocator value as a fixed-width short code.
func EncodeBase62(value uint64) (string, error) {
	if value >= Base62Limit {
		return "", ErrInvalidBase62
	}

	code := [CodeLength]byte{}
	for i := CodeLength - 1; i >= 0; i-- {
		code[i] = base62Alphabet[value%62]
		value /= 62
	}
	return string(code[:]), nil
}

// DecodeBase62 decodes a fixed-width short code into an allocator value.
func DecodeBase62(code string) (uint64, error) {
	if len(code) != CodeLength {
		return 0, ErrInvalidBase62
	}

	var value uint64
	for i := 0; i < len(code); i++ {
		digit := base62Digit(code[i])
		if digit < 0 {
			return 0, ErrInvalidBase62
		}
		value = value*62 + uint64(digit)
	}
	return value, nil
}

func base62Digit(char byte) int {
	switch {
	case char >= '0' && char <= '9':
		return int(char - '0')
	case char >= 'a' && char <= 'z':
		return int(char-'a') + 10
	case char >= 'A' && char <= 'Z':
		return int(char-'A') + 36
	default:
		return -1
	}
}
