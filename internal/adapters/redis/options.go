package redis

import (
	"crypto/tls"
	"os"

	goredis "github.com/redis/go-redis/v9"
)

// OptionsFromEnv builds Redis options for Lambda. MOLLA_REDIS_TLS=1 enables TLS
// (ElastiCache in-transit encryption). MOLLA_REDIS_AUTH is the AUTH token.
func OptionsFromEnv(addr string) *goredis.Options {
	opts := &goredis.Options{
		Addr:     addr,
		Password: os.Getenv("MOLLA_REDIS_AUTH"),
	}
	if os.Getenv("MOLLA_REDIS_TLS") == "1" {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return opts
}
