package rag

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"archivist/internal/models"
	"archivist/internal/storage"
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

func TestEnforceLocalAnswerKeepsOnlyCourseContextLinks(t *testing.T) {
	context := "Section URL: https://course.example/lesson"
	answer := `### Related course links

- [Assigned lesson](https://course.example/lesson)
- [Unverified result](https://outside.example/result)`

	filtered := enforceLocalAnswerWithContext(answer, context)
	if !strings.Contains(filtered, "[Assigned lesson](https://course.example/lesson)") {
		t.Fatalf("verified course link was removed: %q", filtered)
	}
	if strings.Contains(filtered, "https://outside.example/result") {
		t.Fatalf("unverified link remained: %q", filtered)
	}
	if !strings.Contains(filtered, "Unverified result") {
		t.Fatalf("unverified link label should remain readable: %q", filtered)
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

func TestStudyTasksHaveGroundedInstructions(t *testing.T) {
	for _, task := range []StudyTask{
		StudyTaskExplain, StudyTaskSummary, StudyTaskCompare,
		StudyTaskGuide, StudyTaskPlan, StudyTaskPractice, StudyTaskFlashcards,
	} {
		if !ValidStudyTask(task) {
			t.Fatalf("expected %q to be a valid study task", task)
		}
		if instructions := taskInstructions(task); instructions == "" {
			t.Fatalf("study task %q has no prompt instructions", task)
		}
	}
	if ValidStudyTask("invent") {
		t.Fatal("unknown study task should be rejected")
	}
}

func TestAnswerWithFiltersOnlyUsesMatchingMaterials(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "archivist.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.DB.Close()
	if err := store.CreateUser("Personal", "personal@filter.test", "hash", "admin"); err != nil {
		t.Fatal(err)
	}
	user, _ := store.UserByEmail("personal@filter.test")
	if err := store.CreateWorkspace("Study", "STUDY", "", true, user.ID); err != nil {
		t.Fatal(err)
	}
	workspaces, _ := store.Workspaces(user)
	for _, item := range []struct{ name, kind, tags, content string }{
		{"lecture.md", "lecture", "midterm", "Lecture context: momentum is mass times velocity."},
		{"reading.md", "reading", "midterm", "Reading context: this must not be retrieved."},
	} {
		result, err := store.DB.Exec(`INSERT INTO documents(workspace_id,name,path,status,index_version,material_type,tags) VALUES(?,?,?,'ready',1,?,?)`, workspaces[0].ID, item.name, item.name, item.kind, item.tags)
		if err != nil {
			t.Fatal(err)
		}
		documentID, _ := result.LastInsertId()
		if _, err := store.DB.Exec(`INSERT INTO document_chunks(document_id,workspace_id,chunk_index,content,source_name,embedding) VALUES(?,?,?,?,?,?)`, documentID, workspaces[0].ID, 0, item.content, item.name, "[1]"); err != nil {
			t.Fatal(err)
		}
	}
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/embeddings":
			_, _ = w.Write([]byte(`{"embedding":[1]}`))
		case "/api/chat":
			_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"Scoped answer."}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer model.Close()

	service := Service{DB: store.DB, Ollama: models.New(model.URL), ChatModel: "test", EmbedModel: "test", PersonalMode: true}
	answer, sources, err := service.AnswerWithTaskAndFilters(context.Background(), workspaces[0].ID, "What is momentum?", StudyTaskExplain, RetrievalFilters{MaterialType: "lecture", Tag: "midterm"})
	if err != nil || answer != "Scoped answer." {
		t.Fatalf("unexpected filtered answer: %q, %v", answer, err)
	}
	if len(sources) != 1 || sources[0] != "lecture.md" {
		t.Fatalf("expected only lecture source, got %v", sources)
	}
}
