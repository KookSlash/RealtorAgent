package fetch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/config"
	"golang.org/x/net/html"
)

type Page struct {
	URL  string
	HTML []byte
}

type NextPageResolver func(html []byte, baseURL string) string

type RealtorFetcher struct {
	cfg              config.Config
	client           *http.Client
	maxRetries       int
	rateLimit        time.Duration
	nextPageResolver NextPageResolver
}

func NewRealtorFetcher(cfg config.Config, client *http.Client) *RealtorFetcher {
	return NewRealtorFetcherWithResolver(cfg, client, nil)
}

func NewRealtorFetcherWithResolver(cfg config.Config, client *http.Client, resolver NextPageResolver) *RealtorFetcher {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if resolver == nil {
		resolver = discoverNextURL
	}
	return &RealtorFetcher{
		cfg:              cfg,
		client:           client,
		maxRetries:       2,
		rateLimit:        time.Duration(cfg.RateLimitMs) * time.Millisecond,
		nextPageResolver: resolver,
	}
}

func (f *RealtorFetcher) FetchAll(ctx context.Context) ([]Page, error) {
	entrypoint := strings.TrimSpace(f.cfg.SearchEntrypointURL)
	if entrypoint == "" {
		return nil, fmt.Errorf("SEARCH_ENTRYPOINT_URL is required for real scraping")
	}

	maxPages := f.cfg.MaxPages
	if maxPages <= 0 {
		maxPages = 1
	}

	pages := []Page{}
	visited := map[string]bool{}
	current := entrypoint

	for pageIndex := 0; pageIndex < maxPages; pageIndex++ {
		if visited[current] {
			break
		}
		visited[current] = true

		htmlBytes, err := f.fetch(ctx, current)
		if err != nil {
			return pages, err
		}
		pages = append(pages, Page{URL: current, HTML: htmlBytes})

		nextURL := f.nextPageResolver(htmlBytes, current)
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

func (f *RealtorFetcher) fetch(ctx context.Context, targetURL string) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt <= f.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", f.cfg.UserAgent)

		resp, err := f.client.Do(req)
		if err != nil {
			lastErr = err
		} else {
			defer resp.Body.Close()
			if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
				lastErr = fmt.Errorf("server error status %d", resp.StatusCode)
			} else if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
			} else {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, err
				}
				return body, nil
			}
		}

		if attempt < f.maxRetries {
			backoff := time.Duration(attempt+1) * 250 * time.Millisecond
			if err := sleepWithContext(ctx, backoff); err != nil {
				return nil, err
			}
		}
	}

	return nil, lastErr
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func discoverNextURL(htmlBytes []byte, baseURL string) string {
	doc, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		return ""
	}

	if next := findRelNextLink(doc, baseURL); next != "" {
		return next
	}
	if next := findNextAnchor(doc, baseURL); next != "" {
		return next
	}
	if next := findPageNumberLink(doc, baseURL); next != "" {
		return next
	}
	return ""
}

func findRelNextLink(doc *html.Node, baseURL string) string {
	node := findFirstNode(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return false
		}
		if n.Data != "link" && n.Data != "a" {
			return false
		}
		rel, ok := getAttr(n, "rel")
		if !ok {
			return false
		}
		return strings.Contains(strings.ToLower(rel), "next")
	})
	if node == nil {
		return ""
	}
	href, ok := getAttr(node, "href")
	if !ok || href == "" || isHashPagination(href) {
		return ""
	}
	return resolveURL(baseURL, href)
}

func findNextAnchor(doc *html.Node, baseURL string) string {
	node := findFirstNode(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.Data != "a" {
			return false
		}
		if label, ok := getAttr(n, "aria-label"); ok && strings.Contains(strings.ToLower(label), "next") {
			return true
		}
		if text := strings.ToLower(nodeText(n)); text == "next" {
			return true
		}
		return false
	})
	if node == nil {
		return ""
	}
	href, ok := getAttr(node, "href")
	if !ok || href == "" || isHashPagination(href) {
		return ""
	}
	return resolveURL(baseURL, href)
}

func findPageNumberLink(doc *html.Node, baseURL string) string {
	currentPage := currentPageFromURL(baseURL)
	type candidate struct {
		page int
		href string
	}
	candidates := []candidate{}

	nodes := findNodes(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "a"
	})
	for _, node := range nodes {
		href, ok := getAttr(node, "href")
		if !ok || href == "" || isHashPagination(href) {
			continue
		}
		page := parsePageFromAnchor(node, href)
		if page == 0 {
			continue
		}
		if page == currentPage+1 {
			return resolveURL(baseURL, href)
		}
		if page > currentPage {
			candidates = append(candidates, candidate{page: page, href: href})
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	best := candidates[0]
	for _, item := range candidates[1:] {
		if item.page < best.page {
			best = item
		}
	}
	return resolveURL(baseURL, best.href)
}

func parsePageFromAnchor(node *html.Node, href string) int {
	if value, ok := getAttr(node, "data-page"); ok {
		if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			return parsed
		}
	}
	text := strings.TrimSpace(nodeText(node))
	if parsed, err := strconv.Atoi(text); err == nil {
		return parsed
	}
	if label, ok := getAttr(node, "aria-label"); ok {
		if parsed := parsePageNumber(label); parsed > 0 {
			return parsed
		}
	}
	if parsed := pageFromURL(href); parsed > 0 {
		return parsed
	}
	return 0
}

func currentPageFromURL(rawURL string) int {
	if parsed := pageFromURL(rawURL); parsed > 0 {
		return parsed
	}
	return 1
}

func pageFromURL(rawURL string) int {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return 0
	}
	if value := parsed.Query().Get("page"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	if match := regexp.MustCompile(`(?i)page-(\d+)`).FindStringSubmatch(parsed.Path); len(match) > 1 {
		if parsed, err := strconv.Atoi(match[1]); err == nil {
			return parsed
		}
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) > 0 {
		last := segments[len(segments)-1]
		if parsed, err := strconv.Atoi(last); err == nil {
			return parsed
		}
	}
	return 0
}

func parsePageNumber(text string) int {
	match := regexp.MustCompile(`\d+`).FindString(text)
	if match == "" {
		return 0
	}
	parsed, err := strconv.Atoi(match)
	if err != nil {
		return 0
	}
	return parsed
}

func resolveURL(baseURL, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	parsed, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if parsed.IsAbs() {
		return parsed.String()
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return href
	}
	return base.ResolveReference(parsed).String()
}

func isHashPagination(href string) bool {
	parsed, err := url.Parse(href)
	if err != nil {
		return true
	}
	return parsed.Fragment != ""
}

func getAttr(node *html.Node, key string) (string, bool) {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, key) {
			return attr.Val, true
		}
	}
	return "", false
}

func nodeText(node *html.Node) string {
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			builder.WriteString(n.Data)
			builder.WriteString(" ")
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return strings.Join(strings.Fields(builder.String()), " ")
}

func findFirstNode(root *html.Node, predicate func(*html.Node) bool) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if predicate(n) {
			found = n
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
			if found != nil {
				return
			}
		}
	}
	walk(root)
	return found
}

func findNodes(root *html.Node, predicate func(*html.Node) bool) []*html.Node {
	var matches []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if predicate(n) {
			matches = append(matches, n)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return matches
}
