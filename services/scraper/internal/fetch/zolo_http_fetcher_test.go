package fetch

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestZoloNextPageRel(t *testing.T) {
	page1 := readFixture(t, fixturePath(t,
		filepath.Join("..", "..", "testdata", "zolo", "Zolo-page1.html"),
		filepath.Join("..", "..", "testdata", "zolo", "page-1.html"),
	))
	page2 := readFixture(t, fixturePath(t,
		filepath.Join("..", "..", "testdata", "zolo", "zolo-page2.html"),
		filepath.Join("..", "..", "testdata", "zolo", "page-2.html"),
	))

	baseURL := "https://www.zolo.ca/calgary-real-estate"

	expected1 := expectedRelNext(page1, baseURL)
	got1 := extractZoloNextURL(page1, baseURL)
	assertExpectedNext(t, expected1, got1)

	expected2 := expectedRelNext(page2, baseURL)
	got2 := extractZoloNextURL(page2, baseURL)
	assertExpectedNext(t, expected2, got2)
}

func assertExpectedNext(t *testing.T, expected, got string) {
	t.Helper()
	if expected == "" && got != "" {
		t.Fatalf("expected empty next url, got %s", got)
	}
	if expected != "" && got != expected {
		t.Fatalf("expected next url %s, got %s", expected, got)
	}
}

func expectedRelNext(htmlBytes []byte, baseURL string) string {
	doc, err := html.Parse(bytes.NewReader(normalizeZoloHTML(htmlBytes)))
	if err != nil {
		return ""
	}
	node := findFirstNode(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.Data != "link" {
			return false
		}
		rel, ok := getAttr(n, "rel")
		return ok && strings.Contains(strings.ToLower(rel), "next")
	})
	if node == nil {
		return ""
	}
	rel, ok := getAttr(node, "href")
	if !ok {
		return ""
	}
	rel = strings.TrimSpace(rel)
	if rel == "" || isHashPagination(rel) {
		return ""
	}
	return resolveAgainstBase(baseURL, rel)
}

func resolveAgainstBase(baseURL, href string) string {
	parsed, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return ""
	}
	if parsed.IsAbs() {
		return parsed.String()
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	return base.ResolveReference(parsed).String()
}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return data
}

func fixturePath(t *testing.T, candidates ...string) string {
	t.Helper()
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	t.Fatalf("fixture not found: %v", candidates)
	return ""
}
