package core

import (
	"errors"
	"math/rand/v2"
	"testing"
)

var fixedPermutationTestKey = []byte("molla-slice-1-fixed-test-key")

func TestPermutationCommittedVectors(t *testing.T) {
	permuter := mustPermuter(t)
	tests := []struct {
		input uint64
		code  string
	}{
		{0, "UIiAFaQ"},
		{1, "ReHEeU7"},
		{2, "KRKa4SN"},
	}

	for _, test := range tests {
		permuted, err := permuter.Permute(test.input)
		if err != nil {
			t.Fatalf("Permute(%d): %v", test.input, err)
		}
		code, err := EncodeBase62(permuted)
		if err != nil {
			t.Fatalf("EncodeBase62(%d): %v", permuted, err)
		}
		if code != test.code {
			t.Errorf("input %d: code = %q, want %q", test.input, code, test.code)
		}

		decoded, err := DecodeBase62(code)
		if err != nil {
			t.Fatalf("DecodeBase62(%q): %v", code, err)
		}
		inverted, err := permuter.Inverse(decoded)
		if err != nil {
			t.Fatalf("Inverse(%d): %v", decoded, err)
		}
		if inverted != test.input {
			t.Errorf("input %d: inverse = %d", test.input, inverted)
		}
	}
}

func TestPermutationRoundTripAndSampledBijection(t *testing.T) {
	permuter := mustPermuter(t)
	inputs := []uint64{0, Base62Limit - 1}
	random := rand.New(rand.NewPCG(3, 4))
	for range 10_000 {
		inputs = append(inputs, random.Uint64N(Base62Limit))
	}

	seenInputs := make(map[uint64]struct{}, len(inputs))
	seenOutputs := make(map[uint64]struct{}, len(inputs))
	for _, input := range inputs {
		if _, duplicate := seenInputs[input]; duplicate {
			continue
		}
		seenInputs[input] = struct{}{}

		output, err := permuter.Permute(input)
		if err != nil {
			t.Fatalf("Permute(%d): %v", input, err)
		}
		if output >= Base62Limit {
			t.Fatalf("Permute(%d) = %d, outside code space", input, output)
		}
		if _, duplicate := seenOutputs[output]; duplicate {
			t.Fatalf("duplicate output %d", output)
		}
		seenOutputs[output] = struct{}{}

		inverted, err := permuter.Inverse(output)
		if err != nil {
			t.Fatalf("Inverse(%d): %v", output, err)
		}
		if inverted != input {
			t.Fatalf("Inverse(Permute(%d)) = %d", input, inverted)
		}
	}
}

func TestPermutationRejectsInvalidInputs(t *testing.T) {
	if _, err := NewPermuter(nil); !errors.Is(err, ErrInvalidPermutation) {
		t.Fatalf("NewPermuter(nil) error = %v", err)
	}

	permuter := mustPermuter(t)
	if _, err := permuter.Permute(Base62Limit); !errors.Is(err, ErrInvalidPermutation) {
		t.Fatalf("Permute(Base62Limit) error = %v", err)
	}
	if _, err := permuter.Inverse(Base62Limit); !errors.Is(err, ErrInvalidPermutation) {
		t.Fatalf("Inverse(Base62Limit) error = %v", err)
	}
}

func BenchmarkPermuteAndEncode(b *testing.B) {
	permuter, err := NewPermuter(fixedPermutationTestKey)
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < b.N; i++ {
		value, err := permuter.Permute(uint64(i) % Base62Limit)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := EncodeBase62(value); err != nil {
			b.Fatal(err)
		}
	}
}

func mustPermuter(t *testing.T) *Permuter {
	t.Helper()
	permuter, err := NewPermuter(fixedPermutationTestKey)
	if err != nil {
		t.Fatal(err)
	}
	return permuter
}
