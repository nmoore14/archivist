package web

import (
	"strings"
	"testing"
)

func TestMarkdownHTMLRendersFormattingAndRejectsRawHTML(t *testing.T) {
	rendered := string(markdownHTML("# Overview\n\n- **First** item\n- `code`\n\n<script>alert('xss')</script>"))

	for _, expected := range []string{
		"<h1>Overview</h1>",
		"<ul>",
		"<strong>First</strong>",
		"<code>code</code>",
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("expected rendered Markdown to contain %q; got %q", expected, rendered)
		}
	}
	if strings.Contains(rendered, "<script>") {
		t.Fatalf("raw HTML must not be rendered: %q", rendered)
	}
}

func TestMarkdownHTMLBlocksDangerousLinks(t *testing.T) {
	rendered := string(markdownHTML("[unsafe](javascript:alert%281%29)"))
	if strings.Contains(rendered, "href=\"javascript:") {
		t.Fatalf("dangerous URL was rendered: %q", rendered)
	}
}
