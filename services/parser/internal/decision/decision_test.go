package decision

import "testing"

func TestShouldSkipProcessed(t *testing.T) {
	if !ShouldSkipProcessed(true) {
		t.Fatalf("expected processed to be skipped")
	}
	if ShouldSkipProcessed(false) {
		t.Fatalf("expected unprocessed to continue")
	}
}

func TestShouldProcessAttempt(t *testing.T) {
	cases := []struct {
		attempts   int
		maxRetries int
		expect     bool
	}{
		{attempts: 0, maxRetries: 3, expect: true},
		{attempts: 2, maxRetries: 3, expect: true},
		{attempts: 3, maxRetries: 3, expect: false},
		{attempts: 4, maxRetries: 3, expect: false},
		{attempts: 0, maxRetries: 0, expect: false},
	}

	for _, tc := range cases {
		if got := ShouldProcessAttempt(tc.attempts, tc.maxRetries); got != tc.expect {
			t.Fatalf("attempts=%d maxRetries=%d expected %v got %v", tc.attempts, tc.maxRetries, tc.expect, got)
		}
	}
}

func TestShouldInsertHistory(t *testing.T) {
	cases := []struct {
		latest  string
		current string
		expect  bool
	}{
		{latest: "", current: "abc", expect: true},
		{latest: "abc", current: "abc", expect: false},
		{latest: "abc", current: "def", expect: true},
	}

	for _, tc := range cases {
		if got := ShouldInsertHistory(tc.latest, tc.current); got != tc.expect {
			t.Fatalf("latest=%q current=%q expected %v got %v", tc.latest, tc.current, tc.expect, got)
		}
	}
}
