package web

import (
	"bytes"
	"context"
	"crypto/subtle"
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"archivist/internal/app"
	"archivist/internal/auth"
	"archivist/internal/rag"
	"archivist/internal/storage"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

//go:embed templates/*.html
var templateFS embed.FS
var markdown = goldmark.New(goldmark.WithExtensions(extension.GFM))

type Server struct {
	App       *app.App
	templates *template.Template
	mux       *http.ServeMux
}
type View struct {
	Title         string
	User          storage.User
	Workspaces    []storage.Workspace
	Workspace     storage.Workspace
	Documents     []storage.Document
	Users         []storage.User
	Messages      []storage.Message
	Sessions      []storage.ChatSession
	Artifacts     []storage.StudyArtifact
	Flashcard     storage.Flashcard
	ReviewStats   storage.ReviewStats
	SessionID     int64
	Note          storage.Note
	IndexStats    storage.IndexStats
	SearchResults []storage.SearchResult
	LLMSettings   storage.LLMSettings
	Query         string
	MaterialType  string
	MaterialTag   string
	ArtifactKind  string
	ArtifactQuery string
	ActiveTab     string
	PersonalMode  bool
	Error, Notice string
}

func New(a *app.App) *Server {
	if a.Mode == app.ModePersonal {
		if _, err := a.Store.EnsurePersonalUser(); err != nil {
			panic(fmt.Sprintf("initialize personal study mode: %v", err))
		}
	}
	t := template.Must(template.New("").Funcs(template.FuncMap{"initials": func(s string) string {
		p := strings.Fields(s)
		if len(p) == 0 {
			return "A"
		}
		out := string([]rune(p[0])[0])
		if len(p) > 1 {
			out += string([]rune(p[len(p)-1])[0])
		}
		return strings.ToUpper(out)
	}, "markdown": markdownHTML, "formatCount": formatCount}).ParseFS(templateFS, "templates/*.html"))
	s := &Server{App: a, templates: t, mux: http.NewServeMux()}
	s.routes()
	return s
}

func markdownHTML(source string) template.HTML {
	source, math := preserveMarkdownMath(source)
	var rendered bytes.Buffer
	if err := markdown.Convert([]byte(source), &rendered); err != nil {
		return template.HTML(template.HTMLEscapeString(source))
	}
	result := rendered.String()
	for placeholder, expression := range math {
		result = strings.ReplaceAll(result, placeholder, template.HTMLEscapeString(expression))
	}
	return template.HTML(result)
}

func preserveMarkdownMath(source string) (string, map[string]string) {
	preserved := make(map[string]string)
	originalSource := source
	placeholderNumber := 0
	delimiters := []struct {
		open, close string
	}{
		{`$$`, `$$`},
		{`\[`, `\]`},
		{`\(`, `\)`},
		{`$`, `$`},
	}

	for _, delimiter := range delimiters {
		var output strings.Builder
		for {
			start := strings.Index(source, delimiter.open)
			if start < 0 {
				output.WriteString(source)
				break
			}
			endOffset := strings.Index(source[start+len(delimiter.open):], delimiter.close)
			if endOffset < 0 {
				output.WriteString(source)
				break
			}
			end := start + len(delimiter.open) + endOffset + len(delimiter.close)
			expression := source[start:end]
			placeholder := ""
			for {
				placeholder = fmt.Sprintf("ARCHIVISTMATHPLACEHOLDER%dX", placeholderNumber)
				placeholderNumber++
				if !strings.Contains(originalSource, placeholder) {
					break
				}
			}
			preserved[placeholder] = expression
			output.WriteString(source[:start])
			output.WriteString(placeholder)
			source = source[end:]
		}
		source = output.String()
	}
	return source, preserved
}

func formatCount(value int) string {
	raw := strconv.Itoa(value)
	for i := len(raw) - 3; i > 0; i -= 3 {
		raw = raw[:i] + "," + raw[i:]
	}
	return raw
}
func (s *Server) Handler() http.Handler { return s.logging(s.mux) }
func (s *Server) routes() {
	static, _ := fs.Sub(os.DirFS("web/static"), ".")
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	if s.personalMode() {
		s.mux.HandleFunc("/", s.home)
		s.mux.HandleFunc("/dashboard", s.home)
		s.mux.HandleFunc("/ask", s.ask)
		s.mux.HandleFunc("/search", s.search)
		s.mux.HandleFunc("/settings", s.settings)
		s.mux.HandleFunc("/workspaces", s.workspaces)
		s.mux.HandleFunc("/workspaces/new", s.newWorkspace)
		s.mux.HandleFunc("/workspaces/", s.workspaceRoutes)
		return
	}
	s.mux.HandleFunc("/setup", s.setup)
	s.mux.HandleFunc("/login", s.login)
	s.mux.HandleFunc("/logout", s.logout)
	s.mux.HandleFunc("/", s.requireAuth(s.home))
	s.mux.HandleFunc("/dashboard", s.requireAuth(s.home))
	s.mux.HandleFunc("/ask", s.requireAuth(s.ask))
	s.mux.HandleFunc("/search", s.requireAuth(s.search))
	s.mux.HandleFunc("/settings", s.requireAdmin(s.settings))
	s.mux.HandleFunc("/workspaces", s.requireAuth(s.workspaces))
	s.mux.HandleFunc("/workspaces/new", s.requireAdmin(s.newWorkspace))
	s.mux.HandleFunc("/workspaces/", s.requireAuth(s.workspaceRoutes))
	s.mux.HandleFunc("/users", s.requireAdmin(s.users))
	s.mux.HandleFunc("/users/new", s.requireAdmin(s.newUser))
}
func (s *Server) render(w http.ResponseWriter, name string, v View) {
	v.PersonalMode = s.personalMode()
	if err := s.templates.ExecuteTemplate(w, name, v); err != nil {
		log.Printf("render %s: %v", name, err)
	}
}
func (s *Server) current(r *http.Request) (storage.User, error) {
	if s.personalMode() {
		return s.App.Store.EnsurePersonalUser()
	}
	c, err := r.Cookie("archivist_session")
	if err != nil {
		return storage.User{}, err
	}
	return s.App.Store.UserBySession(c.Value)
}
func (s *Server) personalMode() bool { return s.App.Mode == app.ModePersonal }
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.App.Store.UserCount() == 0 {
			http.Redirect(w, r, "/setup", http.StatusSeeOther)
			return
		}
		if _, err := s.current(r); err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}
func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		u, _ := s.current(r)
		if u.Role != "admin" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	})
}
func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	if s.App.Store.UserCount() > 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if r.Method == http.MethodGet {
		s.render(w, "auth.html", View{Title: "Create admin account"})
		return
	}
	name, email, password := strings.TrimSpace(r.FormValue("name")), strings.ToLower(strings.TrimSpace(r.FormValue("email"))), r.FormValue("password")
	if name == "" || email == "" || len(password) < 8 {
		s.render(w, "auth.html", View{Title: "Create admin account", Error: "Enter your name, email, and a password with at least 8 characters."})
		return
	}
	hash, _ := auth.Hash(password)
	if err := s.App.Store.CreateUser(name, email, hash, "admin"); err != nil {
		s.render(w, "auth.html", View{Title: "Create admin account", Error: "That account could not be created."})
		return
	}
	s.signIn(w, r, email, password)
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if s.App.Store.UserCount() == 0 {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	if r.Method == http.MethodGet {
		s.render(w, "auth.html", View{Title: "Welcome back"})
		return
	}
	s.signIn(w, r, strings.ToLower(strings.TrimSpace(r.FormValue("email"))), r.FormValue("password"))
}
func (s *Server) signIn(w http.ResponseWriter, r *http.Request, email, password string) {
	u, err := s.App.Store.UserByEmail(email)
	if err != nil || !auth.Check(u.Password, password) {
		s.render(w, "auth.html", View{Title: "Welcome back", Error: "Email or password is incorrect."})
		return
	}
	token, _ := auth.Token()
	_ = s.App.Store.CreateSession(token, u.ID, time.Now().Add(7*24*time.Hour))
	http.SetCookie(w, &http.Cookie{Name: "archivist_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: time.Now().Add(7 * 24 * time.Hour)})
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, e := r.Cookie("archivist_session"); e == nil {
		s.App.Store.DeleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "archivist_session", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	u, _ := s.current(r)
	ws, _ := s.App.Store.Workspaces(u)
	s.render(w, "dashboard.html", View{Title: "Dashboard", User: u, Workspaces: ws})
}
func (s *Server) ask(w http.ResponseWriter, r *http.Request) {
	u, _ := s.current(r)
	workspaces, _ := s.App.Store.Workspaces(u)
	s.render(w, "ask.html", View{Title: "Ask Archivist", User: u, Workspaces: workspaces})
}
func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	u, _ := s.current(r)
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) > 200 {
		query = query[:200]
	}
	materialType := r.URL.Query().Get("material_type")
	if !validMaterialType(materialType) {
		materialType = ""
	}
	materialTag := strings.TrimSpace(r.URL.Query().Get("tag"))
	if len(materialTag) > 120 {
		materialTag = materialTag[:120]
	}
	results, err := s.App.Store.SearchCourseContentFiltered(u, query, materialType, materialTag, 40)
	if err != nil {
		log.Printf("global search: %v", err)
		s.render(w, "search.html", View{Title: "Search", User: u, Query: query, MaterialType: materialType, MaterialTag: materialTag, Error: "Archivist could not search the course index. Please try again."})
		return
	}
	if materialType == "" && materialTag == "" {
		chatResults, err := s.App.Store.SearchChatHistory(u, query, 40)
		if err != nil {
			log.Printf("chat history search: %v", err)
			s.render(w, "search.html", View{Title: "Search", User: u, Query: query, Error: "Archivist could not search your chat history. Please try again."})
			return
		}
		results = append(chatResults, results...)
	}
	if len(results) > 40 {
		results = results[:40]
	}
	for index := range results {
		results[index].Content = searchExcerpt(results[index].Content, query, 280)
	}
	view := View{Title: "Search", User: u, Query: query, MaterialType: materialType, MaterialTag: materialTag, SearchResults: results}
	if r.URL.Query().Get("partial") == "1" {
		s.render(w, "search-results.html", view)
		return
	}
	s.render(w, "search.html", view)
}

func validMaterialType(materialType string) bool {
	switch materialType {
	case "", "lecture", "reading", "assignment", "reference":
		return true
	default:
		return false
	}
}

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	u, err := s.current(r)
	if err != nil {
		http.Error(w, "Could not load settings", http.StatusInternalServerError)
		return
	}
	if r.Method == http.MethodPost {
		settings, err := validateLLMSettings(storage.LLMSettings{
			BaseURL:    strings.TrimSpace(r.FormValue("ollama_base_url")),
			ChatModel:  strings.TrimSpace(r.FormValue("chat_model")),
			EmbedModel: strings.TrimSpace(r.FormValue("embed_model")),
		})
		if err != nil {
			s.render(w, "settings.html", View{Title: "Settings", User: u, LLMSettings: settings, Error: err.Error()})
			return
		}
		if err := s.App.UpdateLLMSettings(settings); err != nil {
			http.Error(w, "Settings could not be saved.", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/settings?saved=1", http.StatusSeeOther)
		return
	}
	notice := ""
	if r.URL.Query().Get("saved") == "1" {
		notice = "Model connection saved. New questions and indexing jobs will use it."
	}
	s.render(w, "settings.html", View{Title: "Settings", User: u, LLMSettings: s.App.LLMSettings, Notice: notice})
}

func validateLLMSettings(settings storage.LLMSettings) (storage.LLMSettings, error) {
	if len(settings.BaseURL) == 0 || len(settings.BaseURL) > 200 {
		return settings, fmt.Errorf("enter an Ollama server address under 200 characters")
	}
	endpoint, err := url.Parse(settings.BaseURL)
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || (endpoint.Path != "" && endpoint.Path != "/") {
		return settings, fmt.Errorf("use a complete Ollama URL such as http://host.docker.internal:11434")
	}
	settings.BaseURL = strings.TrimRight(settings.BaseURL, "/")
	if len(settings.ChatModel) == 0 || len(settings.ChatModel) > 120 || strings.ContainsAny(settings.ChatModel, "\r\n") {
		return settings, fmt.Errorf("enter a valid chat model name")
	}
	if len(settings.EmbedModel) == 0 || len(settings.EmbedModel) > 120 || strings.ContainsAny(settings.EmbedModel, "\r\n") {
		return settings, fmt.Errorf("enter a valid embedding model name")
	}
	return settings, nil
}

func searchExcerpt(content, query string, limit int) string {
	content = strings.Join(strings.Fields(content), " ")
	if len(content) <= limit {
		return content
	}
	start := strings.Index(strings.ToLower(content), strings.ToLower(query))
	if start < 0 {
		start = 0
	} else {
		start -= limit / 3
		if start < 0 {
			start = 0
		}
	}
	end := start + limit
	if end > len(content) {
		end = len(content)
		start = end - limit
	}
	excerpt := strings.TrimSpace(content[start:end])
	if start > 0 {
		excerpt = "…" + excerpt
	}
	if end < len(content) {
		excerpt += "…"
	}
	return excerpt
}
func (s *Server) workspaces(w http.ResponseWriter, r *http.Request) {
	u, _ := s.current(r)
	if r.Method == http.MethodPost {
		if u.Role != "admin" {
			http.Error(w, "Forbidden", 403)
			return
		}
		pub := r.FormValue("published") == "on" || s.personalMode()
		if err := s.App.Store.CreateWorkspace(strings.TrimSpace(r.FormValue("name")), strings.ToUpper(strings.TrimSpace(r.FormValue("code"))), strings.TrimSpace(r.FormValue("description")), pub, u.ID); err != nil {
			s.render(w, "workspace-new.html", View{Title: "New workspace", User: u, Error: "Use a unique course code and complete the required fields."})
			return
		}
		http.Redirect(w, r, "/workspaces", 303)
		return
	}
	ws, _ := s.App.Store.Workspaces(u)
	s.render(w, "workspaces.html", View{Title: "Workspaces", User: u, Workspaces: ws})
}
func (s *Server) newWorkspace(w http.ResponseWriter, r *http.Request) {
	u, _ := s.current(r)
	s.render(w, "workspace-new.html", View{Title: "New workspace", User: u})
}
func (s *Server) workspaceRoutes(w http.ResponseWriter, r *http.Request) {
	u, _ := s.current(r)
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/workspaces/"), "/"), "/")
	if len(parts) == 0 {
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ws, err := s.App.Store.Workspace(id, u)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	switch action {
	case "":
		http.Redirect(w, r, fmt.Sprintf("/workspaces/%d/chat", id), http.StatusSeeOther)
	case "overview":
		docs, _ := s.App.Store.Documents(id)
		users, _ := s.App.Store.Users()
		s.render(w, "workspace.html", View{Title: ws.Name, User: u, Workspace: ws, Documents: docs, Users: users, ActiveTab: "overview"})
	case "documents":
		s.documents(w, r, u, ws, parts)
	case "index":
		s.courseIndex(w, r, u, ws)
	case "chat":
		s.chat(w, r, u, ws)
	case "notes":
		s.notes(w, r, u, ws, parts)
	case "study":
		s.study(w, r, u, ws)
	case "review":
		s.review(w, r, u, ws)
	case "practice":
		s.practice(w, r, u, ws)
	case "source":
		s.source(w, r, ws)
	case "members":
		if r.Method == http.MethodPost && u.Role == "admin" {
			uid, _ := strconv.ParseInt(r.FormValue("user_id"), 10, 64)
			_ = s.App.Store.AddMember(id, uid)
			http.Redirect(w, r, fmt.Sprintf("/workspaces/%d/overview", id), 303)
		}
	case "reindex":
		if r.Method == http.MethodPost && u.Role == "admin" {
			if err := s.App.Docs.ReindexWorkspace(r.Context(), id); err != nil {
				http.Error(w, "Reindex failed: "+err.Error(), http.StatusUnprocessableEntity)
				return
			}
			http.Redirect(w, r, fmt.Sprintf("/workspaces/%d/documents", id), 303)
		}
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) notes(w http.ResponseWriter, r *http.Request, u storage.User, ws storage.Workspace, parts []string) {
	if len(parts) > 2 && parts[2] == "add" {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		messageID, err := strconv.ParseInt(r.FormValue("message_id"), 10, 64)
		if err != nil || s.App.Store.AddMessageToNote(ws.ID, u.ID, messageID) != nil {
			http.Error(w, "That Archivist response could not be added to your note.", http.StatusUnprocessableEntity)
			return
		}
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<span class="message-action-saved" role="status">Added to note</span>`))
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/workspaces/%d/notes?added=1", ws.ID), http.StatusSeeOther)
		return
	}
	if r.Method == http.MethodPost {
		title := strings.TrimSpace(r.FormValue("title"))
		content := strings.TrimSpace(r.FormValue("content"))
		if len(title) > 120 || len(content) > 1<<20 {
			note, _ := s.App.Store.Note(ws.ID, u.ID)
			s.render(w, "notes.html", View{Title: ws.Name + " notes", User: u, Workspace: ws, Note: note, ActiveTab: "notes", Error: "Keep the title under 120 characters and the note under 1 MB."})
			return
		}
		if err := s.App.Store.SaveNote(ws.ID, u.ID, title, content); err != nil {
			http.Error(w, "Your note could not be saved.", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/workspaces/%d/notes?saved=1", ws.ID), http.StatusSeeOther)
		return
	}
	note, err := s.App.Store.Note(ws.ID, u.ID)
	if err != nil {
		http.Error(w, "Your note could not be loaded.", http.StatusInternalServerError)
		return
	}
	notice := ""
	if r.URL.Query().Get("saved") == "1" {
		notice = "Your note is saved."
	} else if r.URL.Query().Get("added") == "1" {
		notice = "Archivist’s response was added to your note."
	}
	s.render(w, "notes.html", View{Title: ws.Name + " notes", User: u, Workspace: ws, Note: note, ActiveTab: "notes", Notice: notice})
}

func (s *Server) study(w http.ResponseWriter, r *http.Request, u storage.User, ws storage.Workspace) {
	if !s.personalMode() {
		http.NotFound(w, r)
		return
	}
	kind := r.URL.Query().Get("kind")
	if !validArtifactKind(kind) {
		kind = ""
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) > 120 {
		query = query[:120]
	}
	artifacts, err := s.App.Store.StudyArtifactsFiltered(ws.ID, u.ID, kind, query)
	if err != nil {
		http.Error(w, "Saved study material could not be loaded.", http.StatusInternalServerError)
		return
	}
	stats, _ := s.App.Store.ReviewStats(ws.ID, u.ID)
	s.render(w, "study.html", View{Title: ws.Name + " saved study", User: u, Workspace: ws, Artifacts: artifacts, ReviewStats: stats, ArtifactKind: kind, ArtifactQuery: query, ActiveTab: "study"})
}

func validArtifactKind(kind string) bool {
	switch kind {
	case "", "guide", "practice", "flashcards":
		return true
	default:
		return false
	}
}

func (s *Server) review(w http.ResponseWriter, r *http.Request, u storage.User, ws storage.Workspace) {
	if !s.personalMode() {
		http.NotFound(w, r)
		return
	}
	artifactID, _ := strconv.ParseInt(r.FormValue("artifact"), 10, 64)
	if r.Method == http.MethodPost {
		cardID, _ := strconv.ParseInt(r.FormValue("card_id"), 10, 64)
		if cardID == 0 {
			http.Error(w, "Card required", 422)
			return
		}
		if err := s.App.Store.ReviewFlashcard(cardID, u.ID, r.FormValue("result") == "known"); err != nil {
			http.Error(w, "Card could not be reviewed", 500)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/workspaces/%d/review?artifact=%d", ws.ID, artifactID), 303)
		return
	}
	card, err := s.App.Store.DueFlashcard(ws.ID, u.ID, artifactID)
	if err == sql.ErrNoRows {
		s.render(w, "review.html", View{Title: "Review", User: u, Workspace: ws, ActiveTab: "study"})
		return
	}
	if err != nil {
		http.Error(w, "Review could not be loaded", 500)
		return
	}
	s.render(w, "review.html", View{Title: "Review", User: u, Workspace: ws, Flashcard: card, ActiveTab: "study"})
}

func (s *Server) practice(w http.ResponseWriter, r *http.Request, u storage.User, ws storage.Workspace) {
	if !s.personalMode() || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	artifactID, _ := strconv.ParseInt(r.FormValue("artifact_id"), 10, 64)
	outcome := r.FormValue("outcome")
	if artifactID == 0 || (outcome != "got_it" && outcome != "partial" && outcome != "missed") {
		http.Error(w, "Choose how the practice set went.", 422)
		return
	}
	if err := s.App.Store.SavePracticeAttempt(artifactID, u.ID, outcome); err != nil {
		http.Error(w, "Practice result could not be saved.", 500)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/workspaces/%d/study?practiced=1", ws.ID), 303)
}

func (s *Server) courseIndex(w http.ResponseWriter, r *http.Request, u storage.User, ws storage.Workspace) {
	if u.Role != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if r.Method == http.MethodPost {
		retrievalCount, _ := strconv.Atoi(r.FormValue("retrieval_count"))
		profile := r.FormValue("index_profile")
		if err := s.App.Store.UpdateIndexSettings(ws.ID, profile, retrievalCount); err != nil {
			documents, stats, _ := s.App.Store.IndexOverview(ws.ID)
			s.render(w, "index.html", View{Title: ws.Name + " index", User: u, Workspace: ws, Documents: documents, IndexStats: stats, ActiveTab: "index", Error: err.Error()})
			return
		}
		ws, _ = s.App.Store.Workspace(ws.ID, u)
		if err := s.App.Docs.ReindexWorkspace(r.Context(), ws.ID); err != nil {
			documents, stats, _ := s.App.Store.IndexOverview(ws.ID)
			s.render(w, "index.html", View{Title: ws.Name + " index", User: u, Workspace: ws, Documents: documents, IndexStats: stats, ActiveTab: "index", Error: "Settings were saved, but the index could not be rebuilt. Confirm the local model is running, then try again."})
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/workspaces/%d/index?saved=1", ws.ID), http.StatusSeeOther)
		return
	}
	documents, stats, err := s.App.Store.IndexOverview(ws.ID)
	if err != nil {
		http.Error(w, "Could not load the course index", http.StatusInternalServerError)
		return
	}
	notice := ""
	if r.URL.Query().Get("saved") == "1" {
		notice = "Index settings saved and course content rebuilt."
	}
	s.render(w, "index.html", View{Title: ws.Name + " index", User: u, Workspace: ws, Documents: documents, IndexStats: stats, ActiveTab: "index", Notice: notice})
}

func (s *Server) source(w http.ResponseWriter, r *http.Request, ws storage.Workspace) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		http.NotFound(w, r)
		return
	}
	file, err := s.App.Store.DocumentFile(ws.ID, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	contentType := file.MIMEType
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = mime.TypeByExtension(strings.ToLower(filepath.Ext(file.Name)))
	}
	if extension := strings.ToLower(filepath.Ext(file.Name)); extension == ".html" || extension == ".htm" {
		contentType = "text/html; charset=utf-8"
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; style-src 'unsafe-inline'; img-src data:")
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename=%q`, strings.ReplaceAll(file.Name, `"`, "")))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, file.Path)
}
func (s *Server) documents(w http.ResponseWriter, r *http.Request, u storage.User, ws storage.Workspace, parts []string) {
	if len(parts) > 2 && parts[2] == "status" && r.Method == http.MethodGet {
		s.renderDocuments(w, ws)
		return
	}
	if len(parts) > 3 && parts[3] == "delete" && r.Method == http.MethodPost && u.Role == "admin" {
		docID, _ := strconv.ParseInt(parts[2], 10, 64)
		path, err := s.App.Store.DeleteDocument(docID, ws.ID)
		if err == nil {
			_ = os.Remove(path)
		}
		s.renderDocuments(w, ws)
		return
	}
	if r.Method == http.MethodPost && u.Role == "admin" {
		err := r.ParseMultipartForm(128 << 20)
		materialType := r.FormValue("material_type")
		switch materialType {
		case "lecture", "reading", "assignment", "reference":
		default:
			materialType = "reading"
		}
		tags := strings.TrimSpace(r.FormValue("tags"))
		if len(tags) > 120 {
			err = fmt.Errorf("keep tags under 120 characters")
		}
		var headers []*multipart.FileHeader
		if r.MultipartForm != nil {
			headers = r.MultipartForm.File["documents"]
		}
		if err == nil && len(headers) == 0 {
			err = fmt.Errorf("choose at least one course file")
		}
		var documentIDs []int64
		for _, header := range headers {
			file, openErr := header.Open()
			if openErr != nil {
				err = openErr
				break
			}
			documentID, queueErr := s.App.Docs.QueueUploadWithMetadata(ws.ID, file, header, materialType, tags)
			_ = file.Close()
			if queueErr != nil {
				err = queueErr
				break
			}
			documentIDs = append(documentIDs, documentID)
		}
		if err != nil {
			if len(documentIDs) > 0 {
				go s.App.Docs.IndexBatch(context.Background(), documentIDs)
			}
			w.WriteHeader(422)
			_, _ = fmt.Fprintf(w, `<div class="alert error">%s</div>`, template.HTMLEscapeString(err.Error()))
			return
		}
		go s.App.Docs.IndexBatch(context.Background(), documentIDs)
		s.renderDocuments(w, ws)
		return
	}
	docs, _ := s.App.Store.Documents(ws.ID)
	s.render(w, "documents.html", View{Title: "Course library", User: u, Workspace: ws, Documents: docs, ActiveTab: "documents"})
}
func (s *Server) renderDocuments(w http.ResponseWriter, ws storage.Workspace) {
	docs, _ := s.App.Store.Documents(ws.ID)
	s.render(w, "document-list.html", View{Workspace: ws, Documents: docs})
}
func (s *Server) chat(w http.ResponseWriter, r *http.Request, u storage.User, ws storage.Workspace) {
	sessions, err := s.App.Store.ChatSessions(ws.ID, u.ID)
	if err != nil {
		http.Error(w, "Conversations could not be loaded.", http.StatusInternalServerError)
		return
	}
	if r.Method == http.MethodGet && r.URL.Query().Get("new") == "1" {
		id, err := s.App.Store.CreateChatSession(ws.ID, u.ID)
		if err != nil {
			http.Error(w, "Conversation could not be created.", 500)
			return
		}
		if len(sessions) == 0 {
			_ = s.App.Store.AttachLegacyMessages(id, ws.ID, u.ID)
		}
		http.Redirect(w, r, fmt.Sprintf("/workspaces/%d/chat?session=%d", ws.ID, id), http.StatusSeeOther)
		return
	}
	if len(sessions) == 0 {
		id, err := s.App.Store.CreateChatSession(ws.ID, u.ID)
		if err != nil {
			http.Error(w, "Conversation could not be created.", 500)
			return
		}
		if err := s.App.Store.AttachLegacyMessages(id, ws.ID, u.ID); err != nil {
			http.Error(w, "Conversation history could not be migrated.", 500)
			return
		}
		sessions = []storage.ChatSession{{ID: id, Title: "New conversation"}}
	}
	sessionID, _ := strconv.ParseInt(r.FormValue("session_id"), 10, 64)
	if sessionID == 0 {
		sessionID, _ = strconv.ParseInt(r.URL.Query().Get("session"), 10, 64)
	}
	if sessionID == 0 {
		sessionID = sessions[0].ID
	}
	var session storage.ChatSession
	found := false
	for _, candidate := range sessions {
		if candidate.ID == sessionID {
			session = candidate
			found = true
			break
		}
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		q := strings.TrimSpace(r.FormValue("question"))
		if q == "" {
			http.Error(w, "Question required", 422)
			return
		}
		task := rag.StudyTask(r.FormValue("study_task"))
		if task != "" && (!s.personalMode() || !rag.ValidStudyTask(task)) {
			http.Error(w, "Choose a valid study action.", http.StatusUnprocessableEntity)
			return
		}
		filters := rag.RetrievalFilters{}
		if s.personalMode() {
			filters.MaterialType = r.FormValue("material_type")
			if !validMaterialType(filters.MaterialType) {
				http.Error(w, "Choose a valid material type.", http.StatusUnprocessableEntity)
				return
			}
			filters.Tag = strings.TrimSpace(r.FormValue("tag"))
			if len(filters.Tag) > 120 {
				http.Error(w, "Keep the tag under 120 characters.", http.StatusUnprocessableEntity)
				return
			}
		}
		_ = s.App.Store.SaveSessionMessage(sessionID, ws.ID, u.ID, "user", q, "")
		if session.Title == "New conversation" {
			_ = s.App.Store.RenameChatSession(sessionID, u.ID, shortTitle(q, 60))
		}
		answer, sources, err := s.App.RAG.AnswerWithTaskAndFilters(r.Context(), ws.ID, q, task, filters)
		if err != nil {
			answer = "Archivist could not reach the local model. Confirm Ollama is running and both models are installed."
		}
		rawSources := storage.EncodeSources(sources)
		_ = s.App.Store.SaveSessionMessage(sessionID, ws.ID, u.ID, "assistant", answer, rawSources)
		if kind := artifactKind(task); kind != "" {
			artifactID, _ := s.App.Store.CreateStudyArtifact(ws.ID, u.ID, kind, artifactTitle(task, q), answer, rawSources)
			if kind == "flashcards" && artifactID > 0 {
				_ = s.App.Store.CreateFlashcards(artifactID, ws.ID, u.ID, parseFlashcards(answer))
			}
		}
		msgs, _ := s.App.Store.MessagesForSession(sessionID, ws.ID, u.ID)
		s.render(w, "message-list.html", View{Workspace: ws, Messages: msgs})
		return
	}
	msgs, _ := s.App.Store.MessagesForSession(sessionID, ws.ID, u.ID)
	s.render(w, "chat.html", View{Title: ws.Name + " chat", User: u, Workspace: ws, Messages: msgs, Sessions: sessions, SessionID: sessionID, ActiveTab: "chat"})
}

func shortTitle(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len([]rune(value)) > limit {
		return string([]rune(value)[:limit-1]) + "…"
	}
	return value
}
func artifactKind(task rag.StudyTask) string {
	switch task {
	case rag.StudyTaskGuide, rag.StudyTaskPlan:
		return "guide"
	case rag.StudyTaskPractice:
		return "practice"
	case rag.StudyTaskFlashcards:
		return "flashcards"
	}
	return ""
}
func artifactTitle(task rag.StudyTask, question string) string {
	label := map[rag.StudyTask]string{rag.StudyTaskGuide: "Study guide", rag.StudyTaskPlan: "Study plan", rag.StudyTaskPractice: "Practice set", rag.StudyTaskFlashcards: "Flashcards"}[task]
	return label + " — " + shortTitle(question, 60)
}

func parseFlashcards(content string) [][2]string {
	parts := strings.Split(content, "**Front:**")
	cards := make([][2]string, 0, len(parts)-1)
	for _, part := range parts[1:] {
		pair := strings.SplitN(part, "**Back:**", 2)
		if len(pair) != 2 {
			continue
		}
		front, back := strings.TrimSpace(pair[0]), strings.TrimSpace(pair[1])
		if front != "" && back != "" {
			cards = append(cards, [2]string{front, back})
		}
	}
	return cards
}
func (s *Server) users(w http.ResponseWriter, r *http.Request) {
	u, _ := s.current(r)
	users, _ := s.App.Store.Users()
	s.render(w, "users.html", View{Title: "People", User: u, Users: users})
}
func (s *Server) newUser(w http.ResponseWriter, r *http.Request) {
	u, _ := s.current(r)
	if r.Method == http.MethodGet {
		s.render(w, "user-new.html", View{Title: "Add student", User: u})
		return
	}
	hash, _ := auth.Hash(r.FormValue("password"))
	err := s.App.Store.CreateUser(strings.TrimSpace(r.FormValue("name")), strings.ToLower(strings.TrimSpace(r.FormValue("email"))), hash, "student")
	if err != nil {
		s.render(w, "user-new.html", View{Title: "Add student", User: u, Error: "Could not create that student. Check the email is unique."})
		return
	}
	http.Redirect(w, r, "/users", 303)
}
func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

var _ = subtle.ConstantTimeCompare
var _ = sql.ErrNoRows
