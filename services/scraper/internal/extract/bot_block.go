package extract

import (
	"bytes"
	"errors"
	"strings"

	"golang.org/x/net/html"
)

var ErrBlockedByBotDefense = errors.New("blocked by bot defense")

func HasRobotsNoIndex(htmlBytes []byte) bool {
	if len(htmlBytes) == 0 {
		return false
	}
	doc, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		return false
	}
	return hasRobotsNoIndex(doc)
}

func hasRobotsNoIndex(node *html.Node) bool {
	if node.Type == html.ElementNode && strings.EqualFold(node.Data, "meta") {
		name := attrValue(node, "name")
		httpEquiv := attrValue(node, "http-equiv")
		if strings.EqualFold(name, "robots") || strings.EqualFold(httpEquiv, "robots") {
			content := strings.ToLower(attrValue(node, "content"))
			if strings.Contains(content, "noindex") {
				return true
			}
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if hasRobotsNoIndex(child) {
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
