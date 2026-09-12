package core

import (
	"testing"
	"time"
)

func TestDecideRedirect(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		link *RedirectLink
		want RedirectDecision
	}{
		{
			name: "active and not expired",
			link: &RedirectLink{
				LongURL:   "https://example.com/destination",
				IsActive:  true,
				ExpiresAt: now.Add(time.Second),
			},
			want: RedirectDecision{
				Found:   true,
				LongURL: "https://example.com/destination",
			},
		},
		{
			name: "expires exactly now",
			link: &RedirectLink{
				LongURL:   "https://example.com/destination",
				IsActive:  true,
				ExpiresAt: now,
			},
		},
		{
			name: "expired before now",
			link: &RedirectLink{
				LongURL:   "https://example.com/destination",
				IsActive:  true,
				ExpiresAt: now.Add(-time.Second),
			},
		},
		{
			name: "inactive regardless of future expiry",
			link: &RedirectLink{
				LongURL:   "https://example.com/destination",
				IsActive:  false,
				ExpiresAt: now.Add(time.Hour),
			},
		},
		{name: "unknown", link: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := DecideRedirect(test.link, now)
			if got != test.want {
				t.Errorf("DecideRedirect() = %#v, want %#v", got, test.want)
			}
		})
	}
}
