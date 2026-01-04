package fetch

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/config"
	"github.com/KookSlash/RealtorAgent/services/scraper/internal/extract"
	"golang.org/x/net/html"
)

type ZoloHTTPFetcher struct {
	cfg        config.Config
	client     *http.Client
	maxRetries int
	rateLimit  time.Duration
}

func NewZoloHTTPFetcher(cfg config.Config, client *http.Client) (*ZoloHTTPFetcher, error) {
	if client == nil {
		var err error
		client, err = NewHardenedClient(cfg)
		if err != nil {
			return nil, err
		}
	}
	return &ZoloHTTPFetcher{
		cfg:        cfg,
		client:     client,
		maxRetries: 2,
		rateLimit:  time.Duration(cfg.RateLimitMs) * time.Millisecond,
	}, nil
}

func (f *ZoloHTTPFetcher) FetchAll(ctx context.Context) ([]Page, error) {
	entrypoint := strings.TrimSpace(f.cfg.ZoloEntrypointURL)
	if entrypoint == "" {
		return nil, fmt.Errorf("ZOLO_ENTRYPOINT_URL is required for zolo_ca strategy")
	}

	maxPages := f.cfg.MaxPages
	if maxPages <= 0 {
		maxPages = 1
	}

	pages := []Page{}
	visited := map[string]bool{}
	current := entrypoint

	if f.cfg.SeedCookies {
		if err := f.seedCookies(ctx, entrypoint); err != nil {
			return pages, err
		}
	}

	for pageIndex := 0; pageIndex < maxPages; pageIndex++ {
		if visited[current] {
			break
		}
		visited[current] = true

		result, err := f.fetch(ctx, current)
		if err != nil {
			return pages, err
		}
		pageURL := strings.TrimSpace(result.finalURL)
		if pageURL == "" {
			pageURL = current
		}

		if strings.TrimSpace(f.cfg.ScraperSaveHTMLDir) != "" {
			if err := saveHTMLPage(f.cfg.ScraperSaveHTMLDir, pageIndex+1, pageURL, result.body); err != nil {
				return pages, err
			}
		}

		if blocked, reason := DetectBotInterstitial(result.body); blocked {
			return pages, fmt.Errorf("%w: %s url=%s", extract.ErrBlockedByBotDefense, reason, pageURL)
		}

		pages = append(pages, Page{URL: pageURL, HTML: result.body})

		nextURL := extractZoloNextURL(result.body, pageURL)
		if nextURL == "" {
			break
		}

		if f.rateLimit > 0 {
			if err := sleepWithContext(ctx, f.rateLimit); err != nil {
				return pages, err
			}
		}
		current = nextURL
	}

	return pages, nil
}

func (f *ZoloHTTPFetcher) fetch(ctx context.Context, targetURL string) (fetchResult, error) {
	var lastErr error

	for attempt := 0; attempt <= f.maxRetries; attempt++ {
		result, retriable, err := f.fetchOnce(ctx, targetURL)
		if err == nil {
			return result, nil
		}
		if !retriable {
			return fetchResult{}, err
		}
		lastErr = err

		if attempt < f.maxRetries {
			if err := sleepWithContext(ctx, retryBackoff(attempt)); err != nil {
				return fetchResult{}, err
			}
		}
	}

	return fetchResult{}, lastErr
}

func (f *ZoloHTTPFetcher) fetchOnce(ctx context.Context, targetURL string) (fetchResult, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return fetchResult{}, false, err
	}
	applyBrowserHeaders(req, f.cfg)

	resp, err := f.client.Do(req)
	if err != nil {
		return fetchResult{}, shouldRetry(err, 0, f.cfg.RetryForbidden), err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet := readBodySnippet(resp, maxSnippetBytes)
		logNonOKResponse(resp, snippet)
		statusErr := fmt.Errorf("unexpected status %d from %s", resp.StatusCode, resp.Request.URL.String())
		return fetchResult{}, shouldRetry(nil, resp.StatusCode, f.cfg.RetryForbidden), statusErr
	}

	body, err := readResponseBody(resp)
	if err != nil {
		return fetchResult{}, false, err
	}

	return fetchResult{body: body, finalURL: resp.Request.URL.String()}, false, nil
}

func (f *ZoloHTTPFetcher) seedCookies(ctx context.Context, entrypoint string) error {
	seedURL := strings.TrimSpace(f.cfg.ZoloBaseURL)
	if seedURL == "" {
		seedURL = baseURLFromEntrypoint(entrypoint)
	}
	if seedURL == "" {
		return fmt.Errorf("seed cookies: invalid seed url")
	}
	_, err := f.fetch(ctx, seedURL)
	return err
}

func extractZoloNextURL(htmlBytes []byte, baseURL string) string {
	normalized := normalizeZoloHTML(htmlBytes)
	doc, err := html.Parse(bytes.NewReader(normalized))
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
	href, ok := getAttr(node, "href")
	if !ok {
		return ""
	}
	href = strings.TrimSpace(href)
	if href == "" || isHashPagination(href) {
		return ""
	}
	return resolveURL(baseURL, href)
}

func normalizeZoloHTML(htmlBytes []byte) []byte {
	if !looksLikeViewSourceHTML(htmlBytes) {
		return htmlBytes
	}
	doc, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		return htmlBytes
	}
	nodes := findNodes(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "td" && hasClass(n, "line-content")
	})
	if len(nodes) == 0 {
		return htmlBytes
	}
	var builder strings.Builder
	for _, node := range nodes {
		line := strings.TrimSpace(nodeText(node))
		if line == "" {
			continue
		}
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	normalized := builder.String()
	if strings.Contains(normalized, "<html") || strings.Contains(normalized, "<!DOCTYPE") {
		return []byte(normalized)
	}
	return htmlBytes
}

func looksLikeViewSourceHTML(htmlBytes []byte) bool {
	if len(htmlBytes) == 0 {
		return false
	}
	return bytes.Contains(htmlBytes, []byte("line-content")) && bytes.Contains(htmlBytes, []byte("html-tag"))
}

func hasClass(node *html.Node, classSubstring string) bool {
	value, ok := getAttr(node, "class")
	if !ok {
		return false
	}
	return strings.Contains(strings.ToLower(value), strings.ToLower(classSubstring))
}
