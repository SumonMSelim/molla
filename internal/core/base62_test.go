package core

import (
	"errors"
	"math/rand/v2"
	"testing"
)

func TestBase62KnownVectors(t *testing.T) {
	tests := []struct {
		value uint64
		code  string
	}{
		{0, "0000000"},
		{1, "0000001"},
		{61, "000000Z"},
		{62, "0000010"},
		{Base62Limit - 1, "ZZZZZZZ"},
	}

	for _, test := range tests {
		code, err := EncodeBase62(test.value)
		if err != nil {
			t.Fatalf("EncodeBase62(%d): %v", test.value, err)
		}
		if code != test.code {
			t.Errorf("EncodeBase62(%d) = %q, want %q", test.value, code, test.code)
		}

		value, err := DecodeBase62(test.code)
		if err != nil {
			t.Fatalf("DecodeBase62(%q): %v", test.code, err)
		}
		if value != test.value {
			t.Errorf("DecodeBase62(%q) = %d, want %d", test.code, value, test.value)
		}
	}
}

func TestBase62RoundTrip(t *testing.T) {
	random := rand.New(rand.NewPCG(1, 2))
	for range 1_000 {
		want := random.Uint64N(Base62Limit)
		code, err := EncodeBase62(want)
		if err != nil {
			t.Fatal(err)
		}
		got, err := DecodeBase62(code)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("round trip = %d, want %d", got, want)
		}
	}
}

func TestBase62RejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		encode *uint64
		decode string
	}{
		{name: "encode out of range", encode: uint64Pointer(Base62Limit)},
		{name: "short", decode: "000000"},
		{name: "long", decode: "00000000"},
		{name: "invalid character", decode: "000000-"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var err error
			if test.encode != nil {
				_, err = EncodeBase62(*test.encode)
			} else {
				_, err = DecodeBase62(test.decode)
			}
			if !errors.Is(err, ErrInvalidBase62) {
				t.Fatalf("error = %v, want %v", err, ErrInvalidBase62)
			}
		})
	}
}

func uint64Pointer(value uint64) *uint64 {
	return &value
}
