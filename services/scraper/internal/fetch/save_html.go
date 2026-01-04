package fetch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func saveHTMLPage(dir string, index int, pageURL string, html []byte) error {
	if strings.TrimSpace(dir) == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	htmlPath := filepath.Join(dir, fmt.Sprintf("page-%d.html", index))
	if err := os.WriteFile(htmlPath, html, 0o644); err != nil {
		return err
	}
	urlPath := filepath.Join(dir, fmt.Sprintf("page-%d.url", index))
	urlContent := strings.TrimSpace(pageURL)
	if urlContent == "" {
		urlContent = "unknown"
	}
	return os.WriteFile(urlPath, []byte(urlContent+"\n"), 0o644)
}
