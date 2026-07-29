package storage

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestIndexSettingsAndOverview(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "archivist.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.DB.Close()

	if err := store.CreateUser("Admin", "admin@index.test", "hash", "admin"); err != nil {
		t.Fatal(err)
	}
	admin, _ := store.UserByEmail("admin@index.test")
	if err := store.CreateWorkspace("History", "HIST1", "", true, admin.ID); err != nil {
		t.Fatal(err)
	}
	workspaces, _ := store.Workspaces(admin)
	workspaceID := workspaces[0].ID

	if err := store.UpdateIndexSettings(workspaceID, "focused", 6); err != nil {
		t.Fatal(err)
	}
	profile, retrievalCount, err := store.IndexSettings(workspaceID)
	if err != nil || profile != "focused" || retrievalCount != 6 {
		t.Fatalf("unexpected settings: %q %d %v", profile, retrievalCount, err)
	}
	if err := store.UpdateIndexSettings(workspaceID, "developer-mode", 20); err == nil {
		t.Fatal("invalid settings should be rejected")
	}

	result, err := store.DB.Exec(`INSERT INTO documents(workspace_id,name,path,status,index_version)
		VALUES(?,?,?,?,?)`, workspaceID, "lecture.md", "/tmp/lecture.md", "ready", 2)
	if err != nil {
		t.Fatal(err)
	}
	documentID, _ := result.LastInsertId()
	for index, content := range []string{"first searchable passage", "second searchable passage"} {
		if _, err := store.DB.Exec(`INSERT INTO document_chunks(document_id,workspace_id,chunk_index,content,source_name,embedding)
			VALUES(?,?,?,?,?,?)`, documentID, workspaceID, index, content, "lecture.md", "[]"); err != nil {
			t.Fatal(err)
		}
	}
	documents, stats, err := store.IndexOverview(workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(documents) != 1 || documents[0].Chunks != 2 || documents[0].LastIndexedAt == nil {
		t.Fatalf("unexpected document overview: %#v", documents)
	}
	if stats.Documents != 1 || stats.ReadyDocuments != 1 || stats.Passages != 2 || stats.Characters == 0 {
		t.Fatalf("unexpected index stats: %#v", stats)
	}
}

func TestOpenAddsIndexVersionToExistingDocumentsTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archivist.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE documents (
		id INTEGER PRIMARY KEY,
		workspace_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		path TEXT NOT NULL,
		mime_type TEXT,
		status TEXT NOT NULL DEFAULT 'processing',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.DB.Close()

	rows, err := store.DB.Query(`PRAGMA table_info(documents)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, dataType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		if name == "index_version" {
			found = true
		}
	}
	if !found {
		t.Fatal("index_version column was not added")
	}
}

func TestMessagesArePrivateToUserWithinWorkspace(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "archivist.db"))
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
	admin, err := store.UserByEmail("admin@example.test")
	if err != nil {
		t.Fatal(err)
	}
	student, err := store.UserByEmail("student@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateWorkspace("Machine Learning", "ML101", "", true, admin.ID); err != nil {
		t.Fatal(err)
	}
	workspaces, err := store.Workspaces(admin)
	if err != nil {
		t.Fatal(err)
	}
	workspaceID := workspaces[0].ID

	if err := store.SaveMessage(workspaceID, admin.ID, "user", "admin question", ""); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveMessage(workspaceID, admin.ID, "assistant", "admin answer", "chapter.pdf"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveMessage(workspaceID, student.ID, "user", "student question", ""); err != nil {
		t.Fatal(err)
	}

	adminMessages, err := store.Messages(workspaceID, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(adminMessages) != 2 || adminMessages[0].Content != "admin question" || adminMessages[1].Content != "admin answer" {
		t.Fatalf("unexpected admin history: %#v", adminMessages)
	}
	studentMessages, err := store.Messages(workspaceID, student.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(studentMessages) != 1 || studentMessages[0].Content != "student question" {
		t.Fatalf("unexpected student history: %#v", studentMessages)
	}
}

func TestSourcesRoundTripAndLegacyDecode(t *testing.T) {
	encoded := EncodeSources([]string{"Chapter1.pdf, page 3", "notes.md"})
	want := []SourceRef{{Name: "Chapter1.pdf", Page: 3}, {Name: "notes.md"}}
	if got := DecodeSources(encoded); !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch: got %#v want %#v", got, want)
	}

	legacy := "Chapter1.pdf, page 6, Chapter1.pdf, page 3"
	legacyWant := []SourceRef{{Name: "Chapter1.pdf", Page: 6}, {Name: "Chapter1.pdf", Page: 3}}
	if got := DecodeSources(legacy); !reflect.DeepEqual(got, legacyWant) {
		t.Fatalf("legacy decode mismatch: got %#v want %#v", got, legacyWant)
	}
}
