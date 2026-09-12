package core

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

const (
	feistelRounds = 6
	halfBits      = 21
	halfMask      = uint64(1<<halfBits - 1)
)

var ErrInvalidPermutation = errors.New("invalid permutation input")

// Permuter hides allocator order with a keyed permutation over the code space.
type Permuter struct {
	key []byte
}

// NewPermuter returns a Permuter that owns a copy of key.
func NewPermuter(key []byte) (*Permuter, error) {
	if len(key) == 0 {
		return nil, ErrInvalidPermutation
	}
	keyCopy := append([]byte(nil), key...)
	return &Permuter{key: keyCopy}, nil
}

// Permute maps a value bijectively within [0, Base62Limit).
func (p *Permuter) Permute(value uint64) (uint64, error) {
	if p == nil || len(p.key) == 0 || value >= Base62Limit {
		return 0, ErrInvalidPermutation
	}

	value = p.permute42(value)
	for value >= Base62Limit {
		value = p.permute42(value)
	}
	return value, nil
}

// Inverse reverses Permute.
func (p *Permuter) Inverse(value uint64) (uint64, error) {
	if p == nil || len(p.key) == 0 || value >= Base62Limit {
		return 0, ErrInvalidPermutation
	}

	value = p.inverse42(value)
	for value >= Base62Limit {
		value = p.inverse42(value)
	}
	return value, nil
}

func (p *Permuter) permute42(value uint64) uint64 {
	left := value >> halfBits
	right := value & halfMask
	for round := byte(0); round < feistelRounds; round++ {
		left, right = right, left^p.roundFunction(round, right)
	}
	return left<<halfBits | right
}

func (p *Permuter) inverse42(value uint64) uint64 {
	left := value >> halfBits
	right := value & halfMask
	for round := feistelRounds - 1; round >= 0; round-- {
		left, right = right^p.roundFunction(byte(round), left), left
	}
	return left<<halfBits | right
}

func (p *Permuter) roundFunction(round byte, right uint64) uint64 {
	message := [9]byte{round}
	binary.BigEndian.PutUint64(message[1:], right)

	hash := hmac.New(sha256.New, p.key)
	_, _ = hash.Write(message[:])
	sum := hash.Sum(nil)
	return uint64(binary.BigEndian.Uint32(sum[:4])) & halfMask
}
