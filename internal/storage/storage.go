package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct{ DB *sql.DB }

type User struct {
	ID        int64
	Name      string
	Email     string
	Role      string
	Password  string
	CreatedAt time.Time
}

type Workspace struct {
	ID             int64
	Name           string
	Code           string
	Description    string
	Published      bool
	CreatedAt      time.Time
	DocCount       int
	MemberCount    int
	IndexProfile   string
	RetrievalCount int
}

type Document struct {
	ID            int64
	Name          string
	Status        string
	Chunks        int
	Characters    int
	AverageChars  int
	IndexVersion  int
	IndexTotal    int
	IndexComplete int
	CreatedAt     time.Time
	LastIndexedAt *time.Time
}

func (d Document) IndexPercent() int {
	if d.Status == "ready" {
		return 100
	}
	if d.IndexTotal == 0 {
		return 0
	}
	percent := d.IndexComplete * 100 / d.IndexTotal
	if percent > 100 {
		return 100
	}
	if percent < 0 {
		return 0
	}
	return percent
}

func (d Document) IsPending() bool {
	return d.Status == "queued" || d.Status == "preparing" || d.Status == "indexing"
}

type IndexStats struct {
	Documents      int
	ReadyDocuments int
	Passages       int
	Characters     int
	AverageChars   int
	LastIndexedAt  *time.Time
}

type DocumentFile struct {
	Name, Path, MIMEType string
}

type Note struct {
	Title     string
	Content   string
	UpdatedAt time.Time
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	s := &Store{DB: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY, name TEXT NOT NULL, email TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL, role TEXT NOT NULL CHECK(role IN ('admin','student')),
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		expires_at DATETIME NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS workspaces (
		id INTEGER PRIMARY KEY, name TEXT NOT NULL, code TEXT NOT NULL UNIQUE,
		description TEXT NOT NULL DEFAULT '', published INTEGER NOT NULL DEFAULT 0,
		index_profile TEXT NOT NULL DEFAULT 'balanced',
		retrieval_count INTEGER NOT NULL DEFAULT 4,
		created_by INTEGER REFERENCES users(id), created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS workspace_members (
		workspace_id INTEGER NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
		user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		PRIMARY KEY(workspace_id,user_id)
	);
	CREATE TABLE IF NOT EXISTS documents (
		id INTEGER PRIMARY KEY, workspace_id INTEGER NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
		name TEXT NOT NULL, path TEXT NOT NULL, mime_type TEXT, status TEXT NOT NULL DEFAULT 'processing',
		index_version INTEGER NOT NULL DEFAULT 0,
		index_total INTEGER NOT NULL DEFAULT 0,
		index_complete INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS document_chunks (
		id INTEGER PRIMARY KEY, document_id INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
		workspace_id INTEGER NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
		chunk_index INTEGER NOT NULL, content TEXT NOT NULL, source_name TEXT NOT NULL,
		page_number INTEGER, embedding TEXT, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS chat_sessions (
		id INTEGER PRIMARY KEY, workspace_id INTEGER NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
		user_id INTEGER NOT NULL REFERENCES users(id), title TEXT NOT NULL DEFAULT 'New conversation',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS chat_messages (
		id INTEGER PRIMARY KEY, chat_session_id INTEGER REFERENCES chat_sessions(id) ON DELETE CASCADE,
		workspace_id INTEGER NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
		user_id INTEGER REFERENCES users(id), role TEXT NOT NULL, content TEXT NOT NULL,
		sources TEXT, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS course_notes (
		id INTEGER PRIMARY KEY,
		workspace_id INTEGER NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
		user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		title TEXT NOT NULL DEFAULT '',
		content TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(workspace_id,user_id)
	);
	CREATE TABLE IF NOT EXISTS jobs (
		id INTEGER PRIMARY KEY, workspace_id INTEGER REFERENCES workspaces(id) ON DELETE CASCADE,
		kind TEXT NOT NULL, status TEXT NOT NULL, detail TEXT, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS audit_logs (
		id INTEGER PRIMARY KEY, user_id INTEGER, action TEXT NOT NULL, entity_type TEXT,
		entity_id INTEGER, detail TEXT, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_chat_messages_workspace_user_created
		ON chat_messages(workspace_id,user_id,created_at,id);`
	if _, err := s.DB.Exec(schema); err != nil {
		return err
	}
	if err := s.ensureColumn("documents", "index_version", `ALTER TABLE documents ADD COLUMN index_version INTEGER NOT NULL DEFAULT 0`); err != nil {
		return err
	}
	if err := s.ensureColumn("documents", "index_total", `ALTER TABLE documents ADD COLUMN index_total INTEGER NOT NULL DEFAULT 0`); err != nil {
		return err
	}
	if err := s.ensureColumn("documents", "index_complete", `ALTER TABLE documents ADD COLUMN index_complete INTEGER NOT NULL DEFAULT 0`); err != nil {
		return err
	}
	if err := s.ensureColumn("workspaces", "index_profile", `ALTER TABLE workspaces ADD COLUMN index_profile TEXT NOT NULL DEFAULT 'balanced'`); err != nil {
		return err
	}
	return s.ensureColumn("workspaces", "retrieval_count", `ALTER TABLE workspaces ADD COLUMN retrieval_count INTEGER NOT NULL DEFAULT 4`)
}

func (s *Store) ensureColumn(table, column, alter string) error {
	rows, err := s.DB.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	_, err = s.DB.Exec(alter)
	return err
}

func (s *Store) UserCount() int {
	var n int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n
}

func (s *Store) CreateUser(name, email, hash, role string) error {
	_, err := s.DB.Exec(`INSERT INTO users(name,email,password_hash,role) VALUES(?,?,?,?)`, name, email, hash, role)
	return err
}

func (s *Store) UserByEmail(email string) (User, error) {
	var u User
	err := s.DB.QueryRow(`SELECT id,name,email,password_hash,role,created_at FROM users WHERE email=?`, email).
		Scan(&u.ID, &u.Name, &u.Email, &u.Password, &u.Role, &u.CreatedAt)
	return u, err
}

func (s *Store) UserBySession(id string) (User, error) {
	var u User
	err := s.DB.QueryRow(`SELECT u.id,u.name,u.email,u.password_hash,u.role,u.created_at
		FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.id=? AND s.expires_at > CURRENT_TIMESTAMP`, id).
		Scan(&u.ID, &u.Name, &u.Email, &u.Password, &u.Role, &u.CreatedAt)
	return u, err
}

func (s *Store) CreateSession(id string, userID int64, expires time.Time) error {
	_, err := s.DB.Exec(`INSERT INTO sessions(id,user_id,expires_at) VALUES(?,?,?)`,
		id, userID, expires.UTC().Format("2006-01-02 15:04:05"))
	return err
}

func (s *Store) DeleteSession(id string) { _, _ = s.DB.Exec(`DELETE FROM sessions WHERE id=?`, id) }

func (s *Store) CreateWorkspace(name, code, description string, published bool, userID int64) error {
	_, err := s.DB.Exec(`INSERT INTO workspaces(name,code,description,published,created_by) VALUES(?,?,?,?,?)`,
		name, code, description, published, userID)
	return err
}

func (s *Store) Workspaces(user User) ([]Workspace, error) {
	q := `SELECT w.id,w.name,w.code,w.description,w.published,w.created_at,w.index_profile,w.retrieval_count,
		(SELECT COUNT(*) FROM documents d WHERE d.workspace_id=w.id),
		(SELECT COUNT(*) FROM workspace_members m WHERE m.workspace_id=w.id)
		FROM workspaces w`
	args := []any{}
	if user.Role == "student" {
		q += ` JOIN workspace_members wm ON wm.workspace_id=w.id WHERE wm.user_id=? AND w.published=1`
		args = append(args, user.ID)
	}
	q += ` ORDER BY w.created_at DESC`
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Workspace
	for rows.Next() {
		var w Workspace
		if err := rows.Scan(&w.ID, &w.Name, &w.Code, &w.Description, &w.Published, &w.CreatedAt, &w.IndexProfile, &w.RetrievalCount, &w.DocCount, &w.MemberCount); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) Workspace(id int64, user User) (Workspace, error) {
	var w Workspace
	q := `SELECT w.id,w.name,w.code,w.description,w.published,w.created_at,w.index_profile,w.retrieval_count,
		(SELECT COUNT(*) FROM documents d WHERE d.workspace_id=w.id),
		(SELECT COUNT(*) FROM workspace_members m WHERE m.workspace_id=w.id)
		FROM workspaces w WHERE w.id=?`
	err := s.DB.QueryRow(q, id).Scan(&w.ID, &w.Name, &w.Code, &w.Description, &w.Published, &w.CreatedAt, &w.IndexProfile, &w.RetrievalCount, &w.DocCount, &w.MemberCount)
	if err == nil && user.Role == "student" {
		var n int
		err = s.DB.QueryRow(`SELECT COUNT(*) FROM workspace_members WHERE workspace_id=? AND user_id=?`, id, user.ID).Scan(&n)
		if err == nil && (n == 0 || !w.Published) {
			err = sql.ErrNoRows
		}
	}
	return w, err
}

func (s *Store) Documents(workspaceID int64) ([]Document, error) {
	rows, err := s.DB.Query(`SELECT d.id,d.name,d.status,d.created_at,d.index_version,d.index_total,d.index_complete,
		COUNT(c.id),COALESCE(SUM(LENGTH(c.content)),0),COALESCE(CAST(AVG(LENGTH(c.content)) AS INTEGER),0),
		COALESCE(strftime('%Y-%m-%dT%H:%M:%SZ',MAX(c.created_at)),'')
		FROM documents d LEFT JOIN document_chunks c ON c.document_id=d.id
		WHERE d.workspace_id=? GROUP BY d.id ORDER BY d.created_at DESC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var docs []Document
	for rows.Next() {
		var d Document
		var lastIndexed string
		if err := rows.Scan(&d.ID, &d.Name, &d.Status, &d.CreatedAt, &d.IndexVersion, &d.IndexTotal, &d.IndexComplete, &d.Chunks, &d.Characters, &d.AverageChars, &lastIndexed); err != nil {
			return nil, err
		}
		if lastIndexed != "" {
			if indexedAt, err := time.Parse(time.RFC3339, lastIndexed); err == nil {
				d.LastIndexedAt = &indexedAt
			}
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

func (s *Store) IndexOverview(workspaceID int64) ([]Document, IndexStats, error) {
	documents, err := s.Documents(workspaceID)
	if err != nil {
		return nil, IndexStats{}, err
	}
	stats := IndexStats{Documents: len(documents)}
	for _, document := range documents {
		if document.Status == "ready" {
			stats.ReadyDocuments++
		}
		stats.Passages += document.Chunks
		stats.Characters += document.Characters
		if document.LastIndexedAt != nil && (stats.LastIndexedAt == nil || document.LastIndexedAt.After(*stats.LastIndexedAt)) {
			indexedAt := *document.LastIndexedAt
			stats.LastIndexedAt = &indexedAt
		}
	}
	if stats.Passages > 0 {
		stats.AverageChars = stats.Characters / stats.Passages
	}
	return documents, stats, nil
}

func (s *Store) UpdateIndexSettings(workspaceID int64, profile string, retrievalCount int) error {
	if profile != "focused" && profile != "balanced" && profile != "broad" {
		return fmt.Errorf("choose a valid passage detail")
	}
	if retrievalCount != 3 && retrievalCount != 4 && retrievalCount != 6 {
		return fmt.Errorf("choose 3, 4, or 6 passages per answer")
	}
	_, err := s.DB.Exec(`UPDATE workspaces SET index_profile=?,retrieval_count=? WHERE id=?`, profile, retrievalCount, workspaceID)
	return err
}

func (s *Store) IndexSettings(workspaceID int64) (string, int, error) {
	var profile string
	var retrievalCount int
	err := s.DB.QueryRow(`SELECT index_profile,retrieval_count FROM workspaces WHERE id=?`, workspaceID).Scan(&profile, &retrievalCount)
	return profile, retrievalCount, err
}

func (s *Store) DeleteDocument(id, workspaceID int64) (string, error) {
	var path string
	if err := s.DB.QueryRow(`SELECT path FROM documents WHERE id=? AND workspace_id=?`, id, workspaceID).Scan(&path); err != nil {
		return "", err
	}
	_, err := s.DB.Exec(`DELETE FROM documents WHERE id=?`, id)
	return path, err
}

func (s *Store) DocumentFile(workspaceID int64, name string) (DocumentFile, error) {
	var file DocumentFile
	err := s.DB.QueryRow(`SELECT name,path,COALESCE(mime_type,'')
		FROM documents WHERE workspace_id=? AND name=?
		ORDER BY created_at DESC,id DESC LIMIT 1`, workspaceID, name).
		Scan(&file.Name, &file.Path, &file.MIMEType)
	return file, err
}

func (s *Store) Users() ([]User, error) {
	rows, err := s.DB.Query(`SELECT id,name,email,password_hash,role,created_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Password, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) AddMember(workspaceID, userID int64) error {
	_, err := s.DB.Exec(`INSERT OR IGNORE INTO workspace_members(workspace_id,user_id) VALUES(?,?)`, workspaceID, userID)
	return err
}

func (s *Store) SaveMessage(workspaceID, userID int64, role, content, sources string) error {
	_, err := s.DB.Exec(`INSERT INTO chat_messages(workspace_id,user_id,role,content,sources) VALUES(?,?,?,?,?)`, workspaceID, userID, role, content, sources)
	return err
}

type Message struct {
	ID            int64
	Role, Content string
	Sources       []SourceRef
	CreatedAt     time.Time
}

type SourceRef struct {
	Name string `json:"name"`
	Page int    `json:"page,omitempty"`
}

var legacyPagedSource = regexp.MustCompile(`([^,]+?\.(?:pdf|txt|md)), page ([0-9]+)`)

func EncodeSources(sources []string) string {
	refs := make([]SourceRef, 0, len(sources))
	for _, source := range sources {
		source = strings.TrimSpace(source)
		match := legacyPagedSource.FindStringSubmatch(source)
		if len(match) == 3 {
			page, _ := strconv.Atoi(match[2])
			refs = append(refs, SourceRef{Name: strings.TrimSpace(match[1]), Page: page})
		} else if source != "" {
			refs = append(refs, SourceRef{Name: source})
		}
	}
	encoded, _ := json.Marshal(refs)
	return string(encoded)
}

func DecodeSources(raw string) []SourceRef {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var refs []SourceRef
	if json.Unmarshal([]byte(raw), &refs) == nil {
		return refs
	}
	matches := legacyPagedSource.FindAllStringSubmatch(raw, -1)
	for _, match := range matches {
		page, _ := strconv.Atoi(match[2])
		refs = append(refs, SourceRef{Name: strings.TrimSpace(match[1]), Page: page})
	}
	if len(refs) == 0 {
		refs = append(refs, SourceRef{Name: strings.TrimSpace(raw)})
	}
	return refs
}

func (s *Store) Messages(workspaceID, userID int64) ([]Message, error) {
	rows, err := s.DB.Query(`SELECT id,role,content,COALESCE(sources,''),created_at
		FROM chat_messages
		WHERE workspace_id=? AND user_id=?
		ORDER BY created_at,id`, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		var sources string
		if err := rows.Scan(&m.ID, &m.Role, &m.Content, &sources, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.Sources = DecodeSources(sources)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) Note(workspaceID, userID int64) (Note, error) {
	var note Note
	err := s.DB.QueryRow(`SELECT title,content,updated_at FROM course_notes
		WHERE workspace_id=? AND user_id=?`, workspaceID, userID).
		Scan(&note.Title, &note.Content, &note.UpdatedAt)
	if err == sql.ErrNoRows {
		return Note{}, nil
	}
	return note, err
}

func (s *Store) SaveNote(workspaceID, userID int64, title, content string) error {
	_, err := s.DB.Exec(`INSERT INTO course_notes(workspace_id,user_id,title,content)
		VALUES(?,?,?,?)
		ON CONFLICT(workspace_id,user_id) DO UPDATE SET
			title=excluded.title,content=excluded.content,updated_at=CURRENT_TIMESTAMP`,
		workspaceID, userID, title, content)
	return err
}

func (s *Store) AddMessageToNote(workspaceID, userID, messageID int64) error {
	var content, rawSources string
	var createdAt time.Time
	err := s.DB.QueryRow(`SELECT content,COALESCE(sources,''),created_at FROM chat_messages
		WHERE id=? AND workspace_id=? AND user_id=? AND role='assistant'`,
		messageID, workspaceID, userID).Scan(&content, &rawSources, &createdAt)
	if err != nil {
		return err
	}
	note, err := s.Note(workspaceID, userID)
	if err != nil {
		return err
	}
	var excerpt strings.Builder
	if strings.TrimSpace(note.Content) != "" {
		excerpt.WriteString(strings.TrimRight(note.Content, "\n"))
		excerpt.WriteString("\n\n")
	}
	fmt.Fprintf(&excerpt, "## From Archivist · %s\n\n%s", createdAt.Format("Jan 2, 2006"), strings.TrimSpace(content))
	if sources := DecodeSources(rawSources); len(sources) > 0 {
		excerpt.WriteString("\n\nSources: ")
		for index, source := range sources {
			if index > 0 {
				excerpt.WriteString("; ")
			}
			excerpt.WriteString(source.Name)
			if source.Page > 0 {
				fmt.Fprintf(&excerpt, ", page %d", source.Page)
			}
		}
	}
	return s.SaveNote(workspaceID, userID, note.Title, excerpt.String())
}

func ParseID(s string) (int64, error) { var id int64; _, err := fmt.Sscan(s, &id); return id, err }
