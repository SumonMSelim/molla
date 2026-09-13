package dynamodb

import "os"

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func linksTable() string {
	return envOr("MOLLA_LINKS_TABLE", tableLinks)
}

func idempotencyTable() string {
	return envOr("MOLLA_IDEMPOTENCY_TABLE", tableIdempotency)
}

func statsTable() string {
	return envOr("MOLLA_STATS_TABLE", tableStats)
}

func countersTable() string {
	return envOr("MOLLA_COUNTERS_TABLE", tableCounters)
}

func credentialsTable() string {
	return envOr("MOLLA_CREDENTIALS_TABLE", tableCredentials)
}
