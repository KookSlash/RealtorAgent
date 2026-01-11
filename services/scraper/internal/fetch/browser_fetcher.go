package fetch

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/config"
	"github.com/playwright-community/playwright-go"
)

type BrowserFetcher struct {
	cfg              config.Config
	rateLimit        time.Duration
	nextPageResolver NextPageResolver
}

type responseCandidate struct {
	response playwright.Response
	url      string
	status   int
}

const defaultBrowserTimeoutMs = 30000

func NewBrowserFetcher(cfg config.Config, resolver NextPageResolver) (*BrowserFetcher, error) {
	if resolver == nil {
		resolver = discoverNextURL
	}
	return &BrowserFetcher{
		cfg:              cfg,
		rateLimit:        time.Duration(cfg.RateLimitMs) * time.Millisecond,
		nextPageResolver: resolver,
	}, nil
}

func (b *BrowserFetcher) FetchAll(ctx context.Context) ([]Page, error) {
	entrypoint := strings.TrimSpace(b.cfg.SearchEntrypointURL)
	if entrypoint == "" {
		return nil, fmt.Errorf("SEARCH_ENTRYPOINT_URL is required for real scraping")
	}

	maxPages := b.cfg.MaxPages
	unlimited := maxPages <= 0

	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("playwright init failed: %w (install via `go run github.com/playwright-community/playwright-go/cmd/playwright install`)", err)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(b.cfg.BrowserHeadless),
	})
	if err != nil {
		return nil, fmt.Errorf("chromium launch failed: %w (install via `go run github.com/playwright-community/playwright-go/cmd/playwright install chromium`)", err)
	}
	defer browser.Close()

	userAgent := strings.TrimSpace(b.cfg.BrowserUserAgent)
	if userAgent == "" {
		userAgent = b.cfg.UserAgent
	}

	contextOptions := playwright.BrowserNewContextOptions{
		UserAgent: playwright.String(userAgent),
	}

	browserContext, err := browser.NewContext(contextOptions)
	if err != nil {
		return nil, fmt.Errorf("browser context failed: %w", err)
	}
	defer browserContext.Close()

	page, err := browserContext.NewPage()
	if err != nil {
		return nil, fmt.Errorf("browser page failed: %w", err)
	}

	capture := &jsonCapture{}
	responseCh := make(chan responseCandidate, 256)
	page.OnResponse(func(response playwright.Response) {
		if response == nil {
			return
		}
		candidate := responseCandidate{
			response: response,
			url:      response.URL(),
			status:   response.Status(),
		}
		select {
		case responseCh <- candidate:
		default:
		}
	})

	timeoutMs := browserTimeoutMs(b.cfg.BrowserTimeoutMs)
	page.SetDefaultTimeout(float64(timeoutMs))
	page.SetDefaultNavigationTimeout(float64(timeoutMs))

	capture.Reset()
	drainResponseCandidates(responseCh)
	if err := gotoPage(page, entrypoint, b.cfg.Referer, timeoutMs); err != nil {
		return nil, err
	}
	if err := waitAfterNavigation(ctx, b.cfg.BrowserWaitMs); err != nil {
		return nil, err
	}
	captureJSONFromResponses(drainResponseCandidates(responseCh), capture)

	pages := []Page{}
	visited := map[string]bool{}
	var previousURL string
	var previousHTML string

	for pageIndex := 0; unlimited || pageIndex < maxPages; pageIndex++ {
		if err := ctx.Err(); err != nil {
			return pages, err
		}

		currentURL := strings.TrimSpace(page.URL())
		if currentURL == "" {
			currentURL = entrypoint
		}

		htmlContent, err := page.Content()
		if err != nil {
			return pages, fmt.Errorf("page content failed: %w", err)
		}

		if pageIndex > 0 && currentURL == previousURL && htmlContent == previousHTML {
			break
		}

		captureJSONFromResponses(drainResponseCandidates(responseCh), capture)

		previousURL = currentURL
		previousHTML = htmlContent
		visited[currentURL] = true

		htmlBytes := []byte(htmlContent)
		capturedJSON := capture.Best()
		if strings.TrimSpace(b.cfg.ScraperSaveHTMLDir) != "" {
			if err := saveHTMLPage(b.cfg.ScraperSaveHTMLDir, pageIndex+1, currentURL, htmlBytes); err != nil {
				return pages, err
			}
		}
		pages = append(pages, Page{URL: currentURL, HTML: htmlBytes, CapturedJSON: capturedJSON})

		nextURL := b.nextPageResolver(htmlBytes, currentURL)
		if nextURL != "" {
			if visited[nextURL] {
				break
			}
			if b.rateLimit > 0 {
				if err := sleepWithContext(ctx, b.rateLimit); err != nil {
					return pages, err
				}
			}
			capture.Reset()
			drainResponseCandidates(responseCh)
			if err := gotoPage(page, nextURL, currentURL, timeoutMs); err != nil {
				return pages, err
			}
			if err := waitAfterNavigation(ctx, b.cfg.BrowserWaitMs); err != nil {
				return pages, err
			}
			captureJSONFromResponses(drainResponseCandidates(responseCh), capture)
			continue
		}

		capture.Reset()
		drainResponseCandidates(responseCh)
		clicked, err := clickNext(page, timeoutMs)
		if err != nil {
			return pages, err
		}
		if !clicked {
			break
		}
		if b.rateLimit > 0 {
			if err := sleepWithContext(ctx, b.rateLimit); err != nil {
				return pages, err
			}
		}
		if err := waitAfterNavigation(ctx, b.cfg.BrowserWaitMs); err != nil {
			return pages, err
		}
		captureJSONFromResponses(drainResponseCandidates(responseCh), capture)
	}

	return pages, nil
}

func gotoPage(page playwright.Page, targetURL, referer string, timeoutMs int) error {
	options := playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
		Timeout:   playwright.Float(float64(timeoutMs)),
	}
	if strings.TrimSpace(referer) != "" {
		options.Referer = playwright.String(referer)
	}
	if _, err := page.Goto(targetURL, options); err == nil {
		return nil
	}
	options.WaitUntil = playwright.WaitUntilStateLoad
	_, err := page.Goto(targetURL, options)
	return err
}

func clickNext(page playwright.Page, timeoutMs int) (bool, error) {
	selectors := []string{
		"a[rel=next]",
		"a[aria-label*=Next]",
		"a[aria-label*=next]",
		"button[aria-label*=Next]",
		"button[aria-label*=next]",
		"a:has-text(\"Next\")",
		"button:has-text(\"Next\")",
	}

	for _, selector := range selectors {
		locator := page.Locator(selector)
		count, err := locator.Count()
		if err != nil || count == 0 {
			continue
		}
		target := locator.First()
		visible, err := target.IsVisible()
		if err != nil || !visible {
			continue
		}
		enabled, err := target.IsEnabled()
		if err == nil && !enabled {
			continue
		}
		if disabled, _ := target.GetAttribute("aria-disabled"); strings.EqualFold(disabled, "true") {
			continue
		}
		if disabled, _ := target.GetAttribute("disabled"); strings.TrimSpace(disabled) != "" {
			continue
		}
		if err := target.Click(playwright.LocatorClickOptions{
			Timeout: playwright.Float(float64(timeoutMs)),
		}); err != nil {
			return false, err
		}
		if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State:   playwright.LoadStateNetworkidle,
			Timeout: playwright.Float(float64(timeoutMs)),
		}); err != nil {
			return true, err
		}
		return true, nil
	}

	return false, nil
}

func browserTimeoutMs(value int) int {
	if value <= 0 {
		return defaultBrowserTimeoutMs
	}
	return value
}

func waitAfterNavigation(ctx context.Context, waitMs int) error {
	if waitMs <= 0 {
		return nil
	}
	return sleepWithContext(ctx, time.Duration(waitMs)*time.Millisecond)
}

func drainResponseCandidates(ch <-chan responseCandidate) []responseCandidate {
	candidates := []responseCandidate{}
	for {
		select {
		case candidate := <-ch:
			candidates = append(candidates, candidate)
		default:
			return candidates
		}
	}
}

func captureJSONFromResponses(candidates []responseCandidate, capture *jsonCapture) {
	for _, candidate := range candidates {
		if candidate.status < 200 || candidate.status >= 300 {
			continue
		}
		contentType, _ := candidate.response.HeaderValue("content-type")
		if !isLikelyJSONResponse(candidate.url, contentType) {
			continue
		}
		if exceedsMaxBodySize(candidate.response) {
			continue
		}
		body, err := candidate.response.Body()
		if err != nil || len(body) == 0 || len(body) > maxJSONCaptureBytes {
			continue
		}
		capture.Add(body)
	}
}

func exceedsMaxBodySize(response playwright.Response) bool {
	value, err := response.HeaderValue("content-length")
	if err != nil {
		return false
	}
	if strings.TrimSpace(value) == "" {
		return false
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return false
	}
	return parsed > maxJSONCaptureBytes
}
