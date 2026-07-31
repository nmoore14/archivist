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

func TestMarkdownHTMLPreservesLatexForClientRendering(t *testing.T) {
	source := "Inline \\(x = \\frac{a}{b}\\), dollar $x \\, y$, and display:\n\n\\[E = mc^2\\]"
	rendered := string(markdownHTML(source))

	for _, expected := range []string{
		`\(x = \frac{a}{b}\)`,
		`$x \, y$`,
		`\[E = mc^2\]`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("expected rendered Markdown to preserve %q; got %q", expected, rendered)
		}
	}
}

func TestMarkdownHTMLEscapesRawHTMLInsideLatex(t *testing.T) {
	rendered := string(markdownHTML(`\(\text{<script>alert(1)</script>}\)`))
	if strings.Contains(rendered, "<script>") {
		t.Fatalf("raw HTML inside LaTeX must remain escaped: %q", rendered)
	}
	if !strings.Contains(rendered, "&lt;script&gt;") {
		t.Fatalf("expected escaped LaTeX content; got %q", rendered)
	}
}
