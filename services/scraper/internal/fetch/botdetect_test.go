package fetch

import "testing"

func TestDetectBotInterstitialRobotsMeta(t *testing.T) {
	html := []byte(`<html><head><meta name="ROBOTS" content="NOINDEX, NOFOLLOW"></head><body>blocked</body></html>`)
	blocked, reason := DetectBotInterstitial(html)
	if !blocked {
		t.Fatalf("expected blocked to be true")
	}
	if reason == "" {
		t.Fatalf("expected reason to be set")
	}
}

func TestDetectBotInterstitialSmallCaptcha(t *testing.T) {
	html := []byte(`<html><body>captcha required</body></html>`)
	blocked, reason := DetectBotInterstitial(html)
	if !blocked {
		t.Fatalf("expected blocked to be true")
	}
	if reason == "" {
		t.Fatalf("expected reason to be set")
	}
}

func TestDetectBotInterstitialNormalPage(t *testing.T) {
	html := []byte(`<html><head><title>Listings</title></head><body><article class="card-listing"></article></body></html>`)
	blocked, reason := DetectBotInterstitial(html)
	if blocked {
		t.Fatalf("expected blocked to be false, got reason=%s", reason)
	}
}
