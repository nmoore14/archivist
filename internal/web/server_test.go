package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"archivist/internal/app"
	"archivist/internal/storage"
)

func TestSourceViewerRequiresWorkspaceAccessAndServesInline(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "archivist.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.DB.Close()

	if err := store.CreateUser("Admin", "admin@example.test", "hash", "admin"); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateUser("Student", "student@example.test", "hash", "student"); err != nil {
		t.Fatal(err)
	}
	admin, _ := store.UserByEmail("admin@example.test")
	student, _ := store.UserByEmail("student@example.test")
	if err := store.CreateWorkspace("Machine Learning", "ML101", "", true, admin.ID); err != nil {
		t.Fatal(err)
	}
	workspaces, _ := store.Workspaces(admin)
	workspaceID := workspaces[0].ID

	documentPath := filepath.Join(t.TempDir(), "chapter.md")
	if err := os.WriteFile(documentPath, []byte("# Feature vectors"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(`INSERT INTO documents(workspace_id,name,path,mime_type,status,index_version)
		VALUES(?,?,?,?,?,?)`, workspaceID, "chapter.md", documentPath, "text/markdown", "ready", 2); err != nil {
		t.Fatal(err)
	}

	if err := store.CreateSession("admin-token", admin.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession("student-token", student.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UserBySession("admin-token"); err != nil {
		t.Fatalf("admin session lookup failed: %v", err)
	}
	server := New(app.New(store)).Handler()

	adminRequest := httptest.NewRequest(http.MethodGet, "/workspaces/1/source?name=chapter.md", nil)
	adminRequest.AddCookie(&http.Cookie{Name: "archivist_session", Value: "admin-token"})
	adminResponse := httptest.NewRecorder()
	server.ServeHTTP(adminResponse, adminRequest)
	if adminResponse.Code != http.StatusOK {
		t.Fatalf("admin source status: %d", adminResponse.Code)
	}
	if disposition := adminResponse.Header().Get("Content-Disposition"); !strings.Contains(disposition, "inline") {
		t.Fatalf("expected inline disposition, got %q", disposition)
	}
	if body := adminResponse.Body.String(); body != "# Feature vectors" {
		t.Fatalf("unexpected source body: %q", body)
	}

	studentRequest := httptest.NewRequest(http.MethodGet, "/workspaces/1/source?name=chapter.md", nil)
	studentRequest.AddCookie(&http.Cookie{Name: "archivist_session", Value: "student-token"})
	studentResponse := httptest.NewRecorder()
	server.ServeHTTP(studentResponse, studentRequest)
	if studentResponse.Code != http.StatusNotFound {
		t.Fatalf("unassigned student should receive 404, got %d", studentResponse.Code)
	}
}

func TestCourseIndexIsAdminOnlyAndUsesPlainLanguage(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "archivist.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.DB.Close()
	if err := store.CreateUser("Admin", "admin@index.test", "hash", "admin"); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateUser("Student", "student@index.test", "hash", "student"); err != nil {
		t.Fatal(err)
	}
	admin, _ := store.UserByEmail("admin@index.test")
	student, _ := store.UserByEmail("student@index.test")
	if err := store.CreateWorkspace("History", "HIST1", "", true, admin.ID); err != nil {
		t.Fatal(err)
	}
	workspaces, _ := store.Workspaces(admin)
	workspaceID := workspaces[0].ID
	if err := store.AddMember(workspaceID, student.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession("admin-index-token", admin.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession("student-index-token", student.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	handler := New(app.New(store)).Handler()

	adminRequest := httptest.NewRequest(http.MethodGet, "/workspaces/1/index", nil)
	adminRequest.AddCookie(&http.Cookie{Name: "archivist_session", Value: "admin-index-token"})
	adminResponse := httptest.NewRecorder()
	handler.ServeHTTP(adminResponse, adminRequest)
	if adminResponse.Code != http.StatusOK {
		t.Fatalf("admin index status: %d", adminResponse.Code)
	}
	body := adminResponse.Body.String()
	for _, phrase := range []string{"Shape how Archivist reads", "Searchable passages", "Passage detail", "Save settings and rebuild index"} {
		if !strings.Contains(body, phrase) {
			t.Errorf("admin index should contain %q", phrase)
		}
	}

	studentRequest := httptest.NewRequest(http.MethodGet, "/workspaces/1/index", nil)
	studentRequest.AddCookie(&http.Cookie{Name: "archivist_session", Value: "student-index-token"})
	studentResponse := httptest.NewRecorder()
	handler.ServeHTTP(studentResponse, studentRequest)
	if studentResponse.Code != http.StatusForbidden {
		t.Fatalf("student index status: got %d, want 403", studentResponse.Code)
	}
}

func TestAskPagePromotesAvailableCourseChats(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "archivist.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.DB.Close()
	if err := store.CreateUser("Admin", "admin@ask.test", "hash", "admin"); err != nil {
		t.Fatal(err)
	}
	admin, _ := store.UserByEmail("admin@ask.test")
	if err := store.CreateWorkspace("Biology", "BIO101", "Living systems and cells", true, admin.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession("admin-ask-token", admin.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/ask", nil)
	request.AddCookie(&http.Cookie{Name: "archivist_session", Value: "admin-ask-token"})
	response := httptest.NewRecorder()
	New(app.New(store)).Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("ask page status: %d", response.Code)
	}
	body := response.Body.String()
	for _, expected := range []string{"Question desk", "Biology", `/workspaces/1/chat`, "Open chat"} {
		if !strings.Contains(body, expected) {
			t.Errorf("ask page should contain %q", expected)
		}
	}
}

func TestCourseRootRedirectsToChatAndOverviewRemainsAvailable(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "archivist.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.DB.Close()
	if err := store.CreateUser("Admin", "admin@default-chat.test", "hash", "admin"); err != nil {
		t.Fatal(err)
	}
	admin, _ := store.UserByEmail("admin@default-chat.test")
	if err := store.CreateWorkspace("Mathematics", "MATH1", "", true, admin.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession("default-chat-token", admin.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	handler := New(app.New(store)).Handler()

	rootRequest := httptest.NewRequest(http.MethodGet, "/workspaces/1", nil)
	rootRequest.AddCookie(&http.Cookie{Name: "archivist_session", Value: "default-chat-token"})
	rootResponse := httptest.NewRecorder()
	handler.ServeHTTP(rootResponse, rootRequest)
	if rootResponse.Code != http.StatusSeeOther || rootResponse.Header().Get("Location") != "/workspaces/1/chat" {
		t.Fatalf("course root: status=%d location=%q", rootResponse.Code, rootResponse.Header().Get("Location"))
	}

	overviewRequest := httptest.NewRequest(http.MethodGet, "/workspaces/1/overview", nil)
	overviewRequest.AddCookie(&http.Cookie{Name: "archivist_session", Value: "default-chat-token"})
	overviewResponse := httptest.NewRecorder()
	handler.ServeHTTP(overviewResponse, overviewRequest)
	if overviewResponse.Code != http.StatusOK || !strings.Contains(overviewResponse.Body.String(), "Recent sources") {
		t.Fatalf("overview should remain available, status=%d", overviewResponse.Code)
	}

	chatRequest := httptest.NewRequest(http.MethodGet, "/workspaces/1/chat", nil)
	chatRequest.AddCookie(&http.Cookie{Name: "archivist_session", Value: "default-chat-token"})
	chatResponse := httptest.NewRecorder()
	handler.ServeHTTP(chatResponse, chatRequest)
	chatBody := chatResponse.Body.String()
	if chatResponse.Code != http.StatusOK {
		t.Fatalf("chat should be available, status=%d", chatResponse.Code)
	}
	for _, coursePage := range []string{
		`href="/workspaces/1/overview"`,
		`href="/workspaces/1/documents"`,
		`href="/workspaces/1/index"`,
		`href="/workspaces/1/chat"`,
	} {
		if !strings.Contains(chatBody, coursePage) {
			t.Errorf("chat page missing course tab %s", coursePage)
		}
	}
	if strings.Index(chatBody, `href="/workspaces/1/chat"`) > strings.Index(chatBody, `href="/workspaces/1/overview"`) {
		t.Error("Ask Archivist tab should appear before Overview")
	}

	for path, activeLink := range map[string]string{
		"/workspaces/1/chat":      `href="/workspaces/1/chat" class="active" aria-current="page"`,
		"/workspaces/1/notes":     `href="/workspaces/1/notes" class="active" aria-current="page"`,
		"/workspaces/1/overview":  `href="/workspaces/1/overview" class="active" aria-current="page"`,
		"/workspaces/1/documents": `href="/workspaces/1/documents" class="active" aria-current="page"`,
		"/workspaces/1/index":     `href="/workspaces/1/index" class="active" aria-current="page"`,
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.AddCookie(&http.Cookie{Name: "archivist_session", Value: "default-chat-token"})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), activeLink) {
			t.Errorf("%s should mark its own tab active: status=%d", path, response.Code)
		}
		if strings.Count(response.Body.String(), `aria-current="page"`) != 1 {
			t.Errorf("%s should have exactly one active course tab", path)
		}
	}
}

func TestStudentCanAddArchivistResponseToPrivateCourseNote(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "archivist.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.DB.Close()
	if err := store.CreateUser("Admin", "admin@web-notes.test", "hash", "admin"); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateUser("Student", "student@web-notes.test", "hash", "student"); err != nil {
		t.Fatal(err)
	}
	admin, _ := store.UserByEmail("admin@web-notes.test")
	student, _ := store.UserByEmail("student@web-notes.test")
	if err := store.CreateWorkspace("Biology", "BIO1", "", true, admin.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.AddMember(1, student.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession("student-notes-token", student.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveMessage(1, student.ID, "assistant", "Cells divide through mitosis.", storage.EncodeSources([]string{"cells.pdf, page 4"})); err != nil {
		t.Fatal(err)
	}
	messages, _ := store.Messages(1, student.ID)
	handler := New(app.New(store)).Handler()

	chatRequest := httptest.NewRequest(http.MethodGet, "/workspaces/1/chat", nil)
	chatRequest.AddCookie(&http.Cookie{Name: "archivist_session", Value: "student-notes-token"})
	chatResponse := httptest.NewRecorder()
	handler.ServeHTTP(chatResponse, chatRequest)
	for _, expected := range []string{`href="/workspaces/1/notes"`, "Copy", "Add to note"} {
		if !strings.Contains(chatResponse.Body.String(), expected) {
			t.Fatalf("chat response missing %q", expected)
		}
	}

	form := strings.NewReader(fmt.Sprintf("message_id=%d", messages[0].ID))
	addRequest := httptest.NewRequest(http.MethodPost, "/workspaces/1/notes/add", form)
	addRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addRequest.Header.Set("HX-Request", "true")
	addRequest.AddCookie(&http.Cookie{Name: "archivist_session", Value: "student-notes-token"})
	addResponse := httptest.NewRecorder()
	handler.ServeHTTP(addResponse, addRequest)
	if addResponse.Code != http.StatusOK || !strings.Contains(addResponse.Body.String(), "Added to note") {
		t.Fatalf("add-to-note response: status=%d body=%q", addResponse.Code, addResponse.Body.String())
	}

	noteRequest := httptest.NewRequest(http.MethodGet, "/workspaces/1/notes", nil)
	noteRequest.AddCookie(&http.Cookie{Name: "archivist_session", Value: "student-notes-token"})
	noteResponse := httptest.NewRecorder()
	handler.ServeHTTP(noteResponse, noteRequest)
	for _, expected := range []string{"Course notes", "Cells divide through mitosis.", "cells.pdf, page 4", "Only you can see this note"} {
		if !strings.Contains(noteResponse.Body.String(), expected) {
			t.Fatalf("notes page missing %q", expected)
		}
	}
}

func TestApplicationStylesheetIncludesInteractiveUIRules(t *testing.T) {
	stylesheet, err := os.ReadFile(filepath.Join("..", "..", "web", "static", "css", "app.css"))
	if err != nil {
		t.Fatal(err)
	}
	css := string(stylesheet)
	if strings.Contains(css, "var(--serif;") {
		t.Fatal("malformed font variable would prevent later interface rules from loading")
	}
	for _, selector := range []string{".source-drawer{", ".message-actions{", ".note-folio{"} {
		if !strings.Contains(css, selector) {
			t.Fatalf("stylesheet missing interactive UI rule %q", selector)
		}
	}
}
