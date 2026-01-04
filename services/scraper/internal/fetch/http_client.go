package fetch

import (
	"compress/gzip"
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/config"
	"github.com/andybalholm/brotli"
	"golang.org/x/net/publicsuffix"
)

const (
	defaultClientTimeout = 20 * time.Second
	maxSnippetBytes      = 200
)

func NewHardenedClient(cfg config.Config) (*http.Client, error) {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Timeout:   defaultClientTimeout,
		Transport: transport,
		Jar:       jar,
	}, nil
}

func applyBrowserHeaders(req *http.Request, cfg config.Config) {
	req.Header.Set("User-Agent", cfg.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-CA,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	if strings.TrimSpace(cfg.Referer) != "" {
		req.Header.Set("Referer", cfg.Referer)
	}
}

func readResponseBody(resp *http.Response) ([]byte, error) {
	reader, err := decodeBody(resp)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

func readBodySnippet(resp *http.Response, limit int) string {
	if limit <= 0 {
		return ""
	}
	reader, err := decodeBody(resp)
	if err != nil {
		return ""
	}
	defer reader.Close()

	snippet, err := io.ReadAll(io.LimitReader(reader, int64(limit)))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(snippet))
}

func decodeBody(resp *http.Response) (io.ReadCloser, error) {
	encoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	if encoding == "" || encoding == "identity" {
		return resp.Body, nil
	}
	if strings.Contains(encoding, ",") {
		encoding = strings.TrimSpace(strings.Split(encoding, ",")[0])
	}
	switch encoding {
	case "gzip":
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		return reader, nil
	case "deflate":
		reader, err := zlib.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		return reader, nil
	case "br":
		return io.NopCloser(brotli.NewReader(resp.Body)), nil
	default:
		return nil, fmt.Errorf("unsupported content encoding: %s", encoding)
	}
}

func shouldRetry(err error, status int, retryForbidden bool) bool {
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}
		return true
	}
	if status == http.StatusTooManyRequests || status >= 500 {
		return true
	}
	if retryForbidden && status == http.StatusForbidden {
		return true
	}
	return false
}

func retryBackoff(attempt int) time.Duration {
	if attempt < 0 {
		return 0
	}
	base := 250 * time.Millisecond
	maxDelay := 2 * time.Second
	delay := base * time.Duration(1<<attempt)
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}

func logNonOKResponse(resp *http.Response, snippet string) {
	log.Printf(
		"non-200 response status=%d url=%s server=%q set_cookie=%q cf_ray=%q snippet=%q",
		resp.StatusCode,
		resp.Request.URL.String(),
		resp.Header.Get("Server"),
		resp.Header.Get("Set-Cookie"),
		resp.Header.Get("CF-Ray"),
		snippet,
	)
}

func baseURLFromEntrypoint(entrypoint string) string {
	parsed, err := url.Parse(entrypoint)
	if err != nil {
		return ""
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host, Path: "/"}).String()
}
