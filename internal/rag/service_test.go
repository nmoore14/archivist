package rag

import (
	"context"
	"strings"
	"testing"
)

func TestAnswerRejectsWebURLsBeforeCallingLocalModels(t *testing.T) {
	service := Service{}
	for _, question := range []string{
		"Summarize https://example.com/private",
		"What does www.example.com say?",
	} {
		answer, sources, err := service.Answer(context.Background(), 1, question)
		if err != nil {
			t.Fatalf("URL rejection returned an error: %v", err)
		}
		if !strings.Contains(answer, "cannot open or retrieve web addresses") {
			t.Fatalf("unexpected URL rejection: %q", answer)
		}
		if len(sources) != 0 {
			t.Fatalf("URL rejection should not return sources: %v", sources)
		}
	}
}

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

func TestSystemPromptIsKeptInItsOwnFile(t *testing.T) {
	prompt := strings.TrimSpace(systemPrompt)
	if prompt == "" {
		t.Fatal("embedded system prompt is empty")
	}
	if !strings.HasPrefix(prompt, "You are Archivist") {
		t.Fatalf("unexpected system prompt: %q", prompt)
	}
	if strings.Contains(prompt, "Course context:") || strings.Contains(prompt, "Student question:") {
		t.Fatal("request-specific content belongs in the user message, not the system prompt")
	}
}
