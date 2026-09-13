package redis

import (
	"crypto/tls"
	"os"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Redis timeouts are deliberately far below the 3s redirect Lambda timeout so a
// Redis failover leaves headroom for the DynamoDB fallback read and the
// response write. MaxRetries is -1 (go-redis retries disabled): the handler
// owns the fallback decision, not the client.
const (
	dialTimeout  = 150 * time.Millisecond
	readTimeout  = 150 * time.Millisecond
	writeTimeout = 150 * time.Millisecond
	maxRetries   = -1
)

// OptionsFromEnv builds Redis options for Lambda. MOLLA_REDIS_TLS=1 enables TLS
// (ElastiCache in-transit encryption). MOLLA_REDIS_AUTH is the AUTH token.
func OptionsFromEnv(addr string) *goredis.Options {
	opts := &goredis.Options{
		Addr:         addr,
		Password:     os.Getenv("MOLLA_REDIS_AUTH"),
		DialTimeout:  dialTimeout,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		MaxRetries:   maxRetries,
	}
	if os.Getenv("MOLLA_REDIS_TLS") == "1" {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return opts
}
