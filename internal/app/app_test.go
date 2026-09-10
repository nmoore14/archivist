package app

import (
	"path/filepath"
	"testing"

	"archivist/internal/storage"
)

func TestOfficeProfileConfiguresNewWorkspaces(t *testing.T) {
	t.Setenv("ARCHIVIST_PROFILE", "office-kb")
	store, err := storage.Open(filepath.Join(t.TempDir(), "archivist.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.DB.Close()

	application := New(store)
	if application.Profile.ID != "office-kb" {
		t.Fatalf("profile = %q, want office-kb", application.Profile.ID)
	}
	if err := store.CreateUser("Admin", "admin@example.test", "hash", "admin"); err != nil {
		t.Fatal(err)
	}
	admin, _ := store.UserByEmail("admin@example.test")
	if err := store.CreateWorkspace("Operations", "OPS", "", true, admin.ID); err != nil {
		t.Fatal(err)
	}
	workspaces, err := store.Workspaces(admin)
	if err != nil {
		t.Fatal(err)
	}
	if got := workspaces[0]; got.IndexProfile != "broad" || got.RetrievalCount != 6 {
		t.Fatalf("office defaults = %s/%d, want broad/6", got.IndexProfile, got.RetrievalCount)
	}
}
