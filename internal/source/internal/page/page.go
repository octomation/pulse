// Package page holds what the parsers of the sources share: the HTML tree of
// an answer, the text of its nodes and the rules every list follows.
package page

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"go.octolab.org/toolset/pulse/internal/trend"
)

// Parse reads the answer of a source as an HTML tree. An answer without a
// single character is an error: the HTML parser would accept it as an empty
// page.
func Parse(body []byte) (*html.Node, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, errors.New("the answer is empty")
	}
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("the answer is not HTML: %w", err)
	}
	return doc, nil
}

// Find returns the elements under root that match, in the order of the page.
func Find(root *html.Node, match func(*html.Node) bool) []*html.Node {
	var found []*html.Node
	for n := range root.Descendants() {
		if n.Type == html.ElementNode && match(n) {
			found = append(found, n)
		}
	}
	return found
}

// First returns the first element under root that matches, or nil.
func First(root *html.Node, match func(*html.Node) bool) *html.Node {
	for n := range root.Descendants() {
		if n.Type == html.ElementNode && match(n) {
			return n
		}
	}
	return nil
}

// Attr returns the value of an attribute of the element.
func Attr(n *html.Node, key string) (string, bool) {
	for _, attr := range n.Attr {
		if attr.Namespace == "" && attr.Key == key {
			return attr.Val, true
		}
	}
	return "", false
}

// HasClass reports whether the element has the class among its classes.
func HasClass(n *html.Node, class string) bool {
	value, _ := Attr(n, "class")
	for name := range strings.FieldsSeq(value) {
		if name == class {
			return true
		}
	}
	return false
}

// Text returns the text a reader sees in the node, with every run of white
// space as one space. Scripts and styles are not text.
func Text(n *html.Node) string {
	var b strings.Builder
	collect(&b, n)
	return strings.Join(strings.Fields(b.String()), " ")
}

func collect(b *strings.Builder, n *html.Node) {
	switch n.Type {
	case html.TextNode:
		b.WriteString(n.Data)
		return
	case html.ElementNode:
		if n.DataAtom == atom.Script || n.DataAtom == atom.Style {
			return
		}
	default:
	}
	for child := range n.ChildNodes() {
		collect(b, child)
	}
}

// OwnText returns the text of the node itself, without the text of the
// elements inside it.
func OwnText(n *html.Node) string {
	var b strings.Builder
	for child := range n.ChildNodes() {
		if child.Type == html.TextNode {
			b.WriteString(child.Data)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// Period checks the period a page shows against the one that was asked for.
// The marker is what the page said about its period, in its own words.
func Period(marker string, got, want trend.Period) error {
	if got != want {
		return fmt.Errorf("the page shows the %s list (%s), not the %s one", got, marker, want)
	}
	return nil
}

// Rank turns the repositories a parser has read, in the order of the page,
// into the entries of a list. A repository the page repeats stays once, at
// its first position, and the ranks run from one. A page without a
// repository is an error.
func Rank(marker string, entries []trend.Entry) ([]trend.Entry, error) {
	seen := make(map[string]struct{}, len(entries))
	ranked := make([]trend.Entry, 0, len(entries))
	for _, entry := range entries {
		key := entry.Repo.Key()
		if _, repeated := seen[key]; repeated {
			continue
		}
		seen[key] = struct{}{}
		entry.Rank = len(ranked) + 1
		ranked = append(ranked, entry)
	}
	if len(ranked) == 0 {
		return nil, fmt.Errorf("the page has no repositories (%s)", marker)
	}
	return ranked, nil
}
