package service

import (
	"bytes"
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var allowedBlogElements = map[string]struct{}{
	"a":          {},
	"b":          {},
	"blockquote": {},
	"br":         {},
	"div":        {},
	"em":         {},
	"figure":     {},
	"h1":         {},
	"h2":         {},
	"h3":         {},
	"hr":         {},
	"i":          {},
	"img":        {},
	"li":         {},
	"ol":         {},
	"p":          {},
	"s":          {},
	"span":       {},
	"strike":     {},
	"strong":     {},
	"u":          {},
	"ul":         {},
}

var droppedBlogElements = map[string]struct{}{
	"iframe":   {},
	"noscript": {},
	"object":   {},
	"script":   {},
	"style":    {},
	"template": {},
}

func sanitizeBlogHTML(raw string) (string, error) {
	contextNode := &html.Node{
		Type:     html.ElementNode,
		Data:     "div",
		DataAtom: atom.Div,
	}
	nodes, err := html.ParseFragment(strings.NewReader(raw), contextNode)
	if err != nil {
		return "", err
	}

	var output bytes.Buffer
	for _, node := range nodes {
		for _, sanitized := range sanitizeBlogNode(node) {
			if err := html.Render(&output, sanitized); err != nil {
				return "", err
			}
		}
	}
	return strings.TrimSpace(output.String()), nil
}

func sanitizeBlogNode(node *html.Node) []*html.Node {
	switch node.Type {
	case html.TextNode:
		return []*html.Node{{Type: html.TextNode, Data: node.Data}}
	case html.ElementNode:
		tag := strings.ToLower(node.Data)
		if _, drop := droppedBlogElements[tag]; drop {
			return nil
		}
		if _, allowed := allowedBlogElements[tag]; !allowed {
			return sanitizeBlogChildren(node)
		}

		clean := &html.Node{
			Type:     html.ElementNode,
			Data:     tag,
			DataAtom: atom.Lookup([]byte(tag)),
			Attr:     sanitizeBlogAttributes(tag, node.Attr),
		}
		for _, child := range sanitizeBlogChildren(node) {
			clean.AppendChild(child)
		}
		return []*html.Node{clean}
	default:
		return nil
	}
}

func sanitizeBlogChildren(node *html.Node) []*html.Node {
	children := make([]*html.Node, 0)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		children = append(children, sanitizeBlogNode(child)...)
	}
	return children
}

func sanitizeBlogAttributes(tag string, attributes []html.Attribute) []html.Attribute {
	clean := make([]html.Attribute, 0, len(attributes))
	targetBlank := false

	for _, attribute := range attributes {
		key := strings.ToLower(attribute.Key)
		value := strings.TrimSpace(attribute.Val)

		switch tag {
		case "a":
			switch key {
			case "href":
				if safeBlogURL(value, false) {
					clean = append(clean, html.Attribute{Key: "href", Val: value})
				}
			case "target":
				if value == "_blank" {
					targetBlank = true
					clean = append(clean, html.Attribute{Key: "target", Val: "_blank"})
				}
			case "title":
				clean = append(clean, html.Attribute{Key: "title", Val: value})
			}
		case "img":
			switch key {
			case "src":
				if safeBlogURL(value, true) {
					clean = append(clean, html.Attribute{Key: "src", Val: value})
				}
			case "alt":
				clean = append(clean, html.Attribute{Key: "alt", Val: value})
			}
		}
	}

	if tag == "a" && targetBlank {
		clean = append(clean, html.Attribute{Key: "rel", Val: "noopener noreferrer"})
	}
	return clean
}

func safeBlogURL(value string, image bool) bool {
	if value == "" || strings.HasPrefix(value, "//") {
		return false
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}

	scheme := strings.ToLower(parsed.Scheme)
	if image {
		return scheme == "http" || scheme == "https"
	}
	if scheme == "" {
		return true
	}
	return scheme == "http" || scheme == "https" || scheme == "mailto" || scheme == "tel"
}

func hasMeaningfulBlogContent(content string) bool {
	contextNode := &html.Node{
		Type:     html.ElementNode,
		Data:     "div",
		DataAtom: atom.Div,
	}
	nodes, err := html.ParseFragment(strings.NewReader(content), contextNode)
	if err != nil {
		return false
	}

	var meaningful func(*html.Node) bool
	meaningful = func(node *html.Node) bool {
		if node.Type == html.TextNode && strings.TrimSpace(node.Data) != "" {
			return true
		}
		if node.Type == html.ElementNode && node.Data == "img" {
			for _, attribute := range node.Attr {
				if attribute.Key == "src" && attribute.Val != "" {
					return true
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if meaningful(child) {
				return true
			}
		}
		return false
	}

	for _, node := range nodes {
		if meaningful(node) {
			return true
		}
	}
	return false
}
