package rag

import (
	"strings"
	"testing"
)

func TestEnforceLocalAnswerRemovesGeneratedReferencesAndURLs(t *testing.T) {
	answer := `Use the indexed chapter's explanation.

Read [outside documentation](https://example.com/docs) or https://example.com/more.

Source references:
- A book the model remembered
- https://scikit-learn.org/reference`

	filtered := enforceLocalAnswer(answer)
	if strings.Contains(filtered, "http") || strings.Contains(filtered, "Source references") || strings.Contains(filtered, "model remembered") {
		t.Fatalf("unverified references remained in answer: %q", filtered)
	}
	if !strings.Contains(filtered, "outside documentation") {
		t.Fatalf("link label should remain readable: %q", filtered)
	}
}

func TestEnforceLocalAnswerKeepsOrdinaryMarkdown(t *testing.T) {
	answer := "## From the course\n\n- First point\n- **Second point**"
	if filtered := enforceLocalAnswer(answer); filtered != answer {
		t.Fatalf("ordinary grounded Markdown changed: %q", filtered)
	}
}
