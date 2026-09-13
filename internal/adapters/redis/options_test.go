package redis

import (
	"testing"
	"time"
)

func TestOptionsFromEnv(t *testing.T) {
	t.Setenv("MOLLA_REDIS_AUTH", "secret")
	t.Setenv("MOLLA_REDIS_TLS", "")
	opts := OptionsFromEnv("127.0.0.1:6379")
	if opts.Addr != "127.0.0.1:6379" || opts.Password != "secret" || opts.TLSConfig != nil {
		t.Fatalf("opts = %+v", opts)
	}
	t.Setenv("MOLLA_REDIS_TLS", "1")
	opts = OptionsFromEnv("redis.example:6379")
	if opts.TLSConfig == nil {
		t.Fatal("expected TLS")
	}
}

// The redirect Lambda has a 3s timeout and must reach its DynamoDB fallback
// when Redis is failing over, so go-redis defaults (5s dial, 5s read/write,
// 3 retries) must not apply.
func TestOptionsFromEnvTimeoutsFitLambdaBudget(t *testing.T) {
	opts := OptionsFromEnv("127.0.0.1:6379")
	if opts.DialTimeout != 150*time.Millisecond {
		t.Fatalf("DialTimeout = %v", opts.DialTimeout)
	}
	if opts.ReadTimeout != 150*time.Millisecond {
		t.Fatalf("ReadTimeout = %v", opts.ReadTimeout)
	}
	if opts.WriteTimeout != 150*time.Millisecond {
		t.Fatalf("WriteTimeout = %v", opts.WriteTimeout)
	}
	if opts.MaxRetries != -1 {
		t.Fatalf("MaxRetries = %d, want -1 (handler owns retry/fallback)", opts.MaxRetries)
	}
	if total := opts.DialTimeout + opts.ReadTimeout + opts.WriteTimeout; total >= time.Second {
		t.Fatalf("worst-case Redis budget %v leaves too little of the 3s Lambda timeout", total)
	}
}
