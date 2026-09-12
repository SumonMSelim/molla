package core

import "time"

// RedirectLink contains the stored fields needed for a redirect decision.
type RedirectLink struct {
	LongURL   string
	IsActive  bool
	ExpiresAt time.Time
}

// RedirectDecision is the provider-neutral result of a link lookup.
type RedirectDecision struct {
	Found   bool
	LongURL string
}

// DecideRedirect returns not found for unknown, inactive, or expired links.
func DecideRedirect(link *RedirectLink, now time.Time) RedirectDecision {
	if link == nil || !link.IsActive || !now.Before(link.ExpiresAt) {
		return RedirectDecision{}
	}
	return RedirectDecision{Found: true, LongURL: link.LongURL}
}
