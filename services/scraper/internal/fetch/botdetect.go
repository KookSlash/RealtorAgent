package fetch

import (
	"bytes"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

const botDetectSmallHTMLThreshold = 2 * 1024

var robotWord = regexp.MustCompile(`\brobots?\b`)

func DetectBotInterstitial(htmlBytes []byte) (bool, string) {
	trimmed := bytes.TrimSpace(htmlBytes)
	if len(trimmed) == 0 {
		return false, ""
	}

	if hasRobotsNoIndex(trimmed) {
		return true, "robots noindex/nofollow"
	}

	lower := strings.ToLower(string(trimmed))
	if hasBotKeywords(lower) {
		return true, "bot keyword detected"
	}

	if len(trimmed) < botDetectSmallHTMLThreshold {
		if strings.Contains(lower, "<script") || strings.Contains(lower, "window.") {
			return true, "suspicious tiny html"
		}
	}

	return false, ""
}

func hasBotKeywords(lower string) bool {
	keywords := []string{
		"captcha",
		"unusual traffic",
		"access denied",
		"verify you are human",
		"blocked",
		"bot detection",
	}
	for _, keyword := range keywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	if robotWord.MatchString(lower) {
		return true
	}
	return false
}

func hasRobotsNoIndex(htmlBytes []byte) bool {
	doc, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		return false
	}
	return findRobotsMeta(doc)
}

func findRobotsMeta(node *html.Node) bool {
	if node.Type == html.ElementNode && strings.EqualFold(node.Data, "meta") {
		name := attrValue(node, "name")
		httpEquiv := attrValue(node, "http-equiv")
		if strings.EqualFold(name, "robots") || strings.EqualFold(httpEquiv, "robots") {
			content := strings.ToLower(attrValue(node, "content"))
			if strings.Contains(content, "noindex") || strings.Contains(content, "nofollow") {
				return true
			}
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if findRobotsMeta(child) {
			return true
		}
	}
	return false
}

func attrValue(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, key) {
			return strings.TrimSpace(attr.Val)
		}
	}
	return ""
}
