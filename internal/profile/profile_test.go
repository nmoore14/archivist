package profile

import "testing"

func TestLoadOfficeKnowledgeBaseProfile(t *testing.T) {
	p, err := Load("office-kb")
	if err != nil {
		t.Fatal(err)
	}
	if p.Workspace != "team knowledge base" || p.Member != "employee" {
		t.Fatalf("unexpected office vocabulary: %#v", p)
	}
	if p.DefaultIndexProfile != "broad" || p.DefaultRetrievalCount != 6 {
		t.Fatalf("unexpected office retrieval defaults: %#v", p)
	}
	if p.SystemPrompt == "" {
		t.Fatal("office profile must define a RAG prompt")
	}
}

func TestLoadRejectsUnknownProfile(t *testing.T) {
	if _, err := Load("not-a-profile"); err == nil {
		t.Fatal("expected unknown profile error")
	}
}
