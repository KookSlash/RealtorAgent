package strategy

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
)

func parseHTML(data []byte) (*html.Node, error) {
	return html.Parse(bytes.NewReader(data))
}

func getAttr(node *html.Node, key string) (string, bool) {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, key) {
			return attr.Val, true
		}
	}
	return "", false
}

func hasClass(node *html.Node, classSubstring string) bool {
	value, ok := getAttr(node, "class")
	if !ok {
		return false
	}
	return strings.Contains(strings.ToLower(value), strings.ToLower(classSubstring))
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
