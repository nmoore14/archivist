package documents

import (
	"context"
	"database/sql"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"archivist/internal/models"
	"archivist/internal/storage"
	_ "github.com/mattn/go-sqlite3"
)

func TestQueueUploadAndIndexDocumentTracksProgress(t *testing.T) {
	modelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"embedding":[0.1,0.2,0.3]}`))
	}))
	defer modelServer.Close()

	store, err := storage.Open(filepath.Join(t.TempDir(), "archivist.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.DB.Close()
	if err := store.CreateUser("Admin", "admin@batch.test", "hash", "admin"); err != nil {
		t.Fatal(err)
	}
	admin, _ := store.UserByEmail("admin@batch.test")
	if err := store.CreateWorkspace("Biology", "BIO1", "", true, admin.ID); err != nil {
		t.Fatal(err)
	}
	workspaces, _ := store.Workspaces(admin)
	service := Service{DB: store.DB, UploadPath: t.TempDir(), EmbedModel: "test", Ollama: models.New(modelServer.URL)}

	var ids []int64
	for index := 0; index < 2; index++ {
		path := filepath.Join(t.TempDir(), fmt.Sprintf("upload-%d.md", index))
		if err := os.WriteFile(path, []byte(strings.Repeat("Cell structure and function. ", 70)), 0o600); err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		id, err := service.QueueUpload(workspaces[0].ID, file, &multipart.FileHeader{Filename: "lesson.md"})
		_ = file.Close()
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if ids[0] == ids[1] {
		t.Fatal("batch files should create separate document records")
	}
	service.IndexBatch(context.Background(), ids)

	documents, err := store.Documents(workspaces[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(documents) != 2 {
		t.Fatalf("got %d documents, want 2", len(documents))
	}
	for _, document := range documents {
		if document.Status != "ready" || document.IndexTotal < 2 || document.IndexComplete != document.IndexTotal || document.IndexPercent() != 100 {
			t.Fatalf("unexpected completed document: %#v", document)
		}
	}
}

func TestChunkSettingsUseCourseProfile(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE workspaces (id INTEGER PRIMARY KEY,index_profile TEXT NOT NULL);
		INSERT INTO workspaces(id,index_profile) VALUES(1,'focused'),(2,'balanced'),(3,'broad')`); err != nil {
		t.Fatal(err)
	}
	service := Service{DB: db}
	tests := []struct {
		workspaceID   int64
		size, overlap int
	}{{1, 700, 120}, {2, 1000, 200}, {3, 1400, 250}, {99, 1000, 200}}
	for _, test := range tests {
		size, overlap := service.chunkSettings(test.workspaceID)
		if size != test.size || overlap != test.overlap {
			t.Errorf("workspace %d: got %d/%d, want %d/%d", test.workspaceID, size, overlap, test.size, test.overlap)
		}
	}
}

func TestChunksForTextPreservesPageNumber(t *testing.T) {
	page := 7
	chunks, err := chunksForText(strings.Repeat("feature vector content ", 90), &page)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for _, chunk := range chunks {
		if chunk.PageNumber == nil || *chunk.PageNumber != page {
			t.Fatalf("expected page %d, got %#v", page, chunk.PageNumber)
		}
		if strings.TrimSpace(chunk.Content) == "" {
			t.Fatal("empty chunk")
		}
	}
}

func TestExtractTextNormalizesWhitespace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(path, []byte("Feature\n\nvector\tcontains   measurements."), 0o600); err != nil {
		t.Fatal(err)
	}
	chunks, err := extract(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := chunks[0].Content; got != "Feature vector contains measurements." {
		t.Fatalf("unexpected content: %q", got)
	}
	if chunks[0].PageNumber != nil {
		t.Fatal("plain-text chunk should not have a page number")
	}
}

func TestExtractPDFRejectsInvalidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.pdf")
	if err := os.WriteFile(path, []byte("not a pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := extractPDF(path)
	if err == nil || !strings.Contains(err.Error(), "could not read PDF") {
		t.Fatalf("expected a PDF read error, got %v", err)
	}
}
