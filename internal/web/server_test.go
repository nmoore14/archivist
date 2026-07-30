package web

import (
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
}
