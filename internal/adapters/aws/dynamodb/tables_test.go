package dynamodb

import "testing"

func TestEnvOrTableNames(t *testing.T) {
	if linksTable() != tableLinks {
		t.Fatalf("default links = %q", linksTable())
	}
	t.Setenv("MOLLA_LINKS_TABLE", "molla-dev-links")
	t.Setenv("MOLLA_IDEMPOTENCY_TABLE", "molla-dev-idempotency")
	t.Setenv("MOLLA_STATS_TABLE", "molla-dev-stats")
	t.Setenv("MOLLA_COUNTERS_TABLE", "molla-dev-counters")
	t.Setenv("MOLLA_CREDENTIALS_TABLE", "molla-dev-credentials")
	if got := linksTable(); got != "molla-dev-links" {
		t.Fatalf("links = %q", got)
	}
	if got := idempotencyTable(); got != "molla-dev-idempotency" {
		t.Fatalf("idempotency = %q", got)
	}
	if got := statsTable(); got != "molla-dev-stats" {
		t.Fatalf("stats = %q", got)
	}
	if got := countersTable(); got != "molla-dev-counters" {
		t.Fatalf("counters = %q", got)
	}
	if got := credentialsTable(); got != "molla-dev-credentials" {
		t.Fatalf("credentials = %q", got)
	}
}
