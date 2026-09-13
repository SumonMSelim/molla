package redis

import "testing"

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
