package fetch

import "testing"

func TestSelectBestJSONCandidate(t *testing.T) {
	candidate1 := []byte(`{"Results":[{"Id":"1"}]}`)
	candidate2 := []byte(`{"Results":[{"Id":"1"},{"Id":"2"}]}`)
	candidate3 := []byte(`{"foo":"bar"}`)

	best, ok := selectBestJSONCandidate([][]byte{candidate1, candidate2, candidate3})
	if !ok {
		t.Fatalf("expected best candidate to be selected")
	}
	if string(best) != string(candidate2) {
		t.Fatalf("expected candidate2 to win, got %s", string(best))
	}
}

func TestSelectBestJSONCandidateNone(t *testing.T) {
	best, ok := selectBestJSONCandidate([][]byte{[]byte(`{"foo":"bar"}`)})
	if ok || best != nil {
		t.Fatalf("expected no candidate to be selected")
	}
}
