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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"archivist/internal/app"
	"archivist/internal/auth"
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
	Note          storage.Note
	IndexStats    storage.IndexStats
	ActiveTab     string
	Error, Notice string
}

func New(a *app.App) *Server {
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
	s.mux.HandleFunc("/setup", s.setup)
	s.mux.HandleFunc("/login", s.login)
	s.mux.HandleFunc("/logout", s.logout)
	s.mux.HandleFunc("/", s.requireAuth(s.home))
	s.mux.HandleFunc("/dashboard", s.requireAuth(s.home))
	s.mux.HandleFunc("/ask", s.requireAuth(s.ask))
	s.mux.HandleFunc("/workspaces", s.requireAuth(s.workspaces))
	s.mux.HandleFunc("/workspaces/new", s.requireAdmin(s.newWorkspace))
	s.mux.HandleFunc("/workspaces/", s.requireAuth(s.workspaceRoutes))
	s.mux.HandleFunc("/users", s.requireAdmin(s.users))
	s.mux.HandleFunc("/users/new", s.requireAdmin(s.newUser))
}
func (s *Server) render(w http.ResponseWriter, name string, v View) {
	if err := s.templates.ExecuteTemplate(w, name, v); err != nil {
		log.Printf("render %s: %v", name, err)
	}
}
func (s *Server) current(r *http.Request) (storage.User, error) {
	c, err := r.Cookie("archivist_session")
	if err != nil {
		return storage.User{}, err
	}
	return s.App.Store.UserBySession(c.Value)
}
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
func (s *Server) workspaces(w http.ResponseWriter, r *http.Request) {
	u, _ := s.current(r)
	if r.Method == http.MethodPost {
		if u.Role != "admin" {
			http.Error(w, "Forbidden", 403)
			return
		}
		pub := r.FormValue("published") == "on"
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
			documentID, queueErr := s.App.Docs.QueueUpload(ws.ID, file, header)
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
	if r.Method == http.MethodPost {
		q := strings.TrimSpace(r.FormValue("question"))
		if q == "" {
			http.Error(w, "Question required", 422)
			return
		}
		_ = s.App.Store.SaveMessage(ws.ID, u.ID, "user", q, "")
		answer, sources, err := s.App.RAG.Answer(r.Context(), ws.ID, q)
		if err != nil {
			answer = "Archivist could not reach the local model. Confirm Ollama is running and both models are installed."
		}
		_ = s.App.Store.SaveMessage(ws.ID, u.ID, "assistant", answer, storage.EncodeSources(sources))
		msgs, _ := s.App.Store.Messages(ws.ID, u.ID)
		s.render(w, "message-list.html", View{Workspace: ws, Messages: msgs})
		return
	}
	msgs, _ := s.App.Store.Messages(ws.ID, u.ID)
	s.render(w, "chat.html", View{Title: ws.Name + " chat", User: u, Workspace: ws, Messages: msgs, ActiveTab: "chat"})
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
