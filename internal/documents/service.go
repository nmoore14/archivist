package documents

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"archivist/internal/models"
	"github.com/ledongthuc/pdf"
)

type Service struct {
	DB                     *sql.DB
	UploadPath, EmbedModel string
	Ollama                 *models.Client
}

type extractedChunk struct {
	Content    string
	PageNumber *int
}

type indexedChunk struct {
	extractedChunk
	Embedding string
}

const CurrentIndexVersion = 2

var (
	htmlIgnoredContent = regexp.MustCompile(`(?is)<(?:script|style|noscript|svg)\b[^>]*>.*?</(?:script|style|noscript|svg)\s*>`)
	htmlComments       = regexp.MustCompile(`(?s)<!--.*?-->`)
	htmlTags           = regexp.MustCompile(`(?s)<[^>]*>`)
)

func (s *Service) Ingest(ctx context.Context, workspaceID int64, file multipart.File, header *multipart.FileHeader) error {
	documentID, err := s.QueueUpload(workspaceID, file, header)
	if err != nil {
		return err
	}
	return s.IndexDocument(ctx, documentID)
}

func (s *Service) QueueUpload(workspaceID int64, file multipart.File, header *multipart.FileHeader) (int64, error) {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".txt" && ext != ".md" && ext != ".html" && ext != ".htm" && ext != ".pdf" {
		return 0, fmt.Errorf("%s: supported file types are .txt, .md, .html, .htm, and .pdf", filepath.Base(header.Filename))
	}
	dir := filepath.Join(s.UploadPath, fmt.Sprint(workspaceID))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0, err
	}
	name := filepath.Base(header.Filename)
	dst, err := os.CreateTemp(dir, "source-*"+ext)
	if err != nil {
		return 0, err
	}
	path := dst.Name()
	if _, err = io.Copy(dst, file); err != nil {
		dst.Close()
		_ = os.Remove(path)
		return 0, err
	}
	if err := dst.Close(); err != nil {
		_ = os.Remove(path)
		return 0, err
	}
	result, err := s.DB.Exec(`INSERT INTO documents(workspace_id,name,path,mime_type,status,index_version,index_total,index_complete)
		VALUES(?,?,?,?,?,?,0,0)`, workspaceID, name, path, header.Header.Get("Content-Type"), "queued", 0)
	if err != nil {
		_ = os.Remove(path)
		return 0, err
	}
	return result.LastInsertId()
}

func (s *Service) IndexDocument(ctx context.Context, documentID int64) error {
	documents, err := s.documentsForReindex(`id=?`, documentID)
	if err != nil {
		return err
	}
	if len(documents) != 1 {
		return fmt.Errorf("queued document not found")
	}
	return s.reindexDocument(ctx, documents[0])
}

func (s *Service) IndexBatch(ctx context.Context, documentIDs []int64) {
	for _, documentID := range documentIDs {
		_ = s.IndexDocument(ctx, documentID)
	}
}

func (s *Service) prepareIndex(ctx context.Context, documentID int64, name string, chunks []extractedChunk) ([]indexedChunk, error) {
	prepared := make([]indexedChunk, 0, len(chunks))
	for i, chunk := range chunks {
		embedding, err := s.Ollama.GenerateEmbedding(ctx, s.EmbedModel, chunk.Content)
		if err != nil {
			return nil, fmt.Errorf("could not index %q: Ollama embedding failed: %w", name, err)
		}
		b, _ := json.Marshal(embedding)
		prepared = append(prepared, indexedChunk{extractedChunk: chunk, Embedding: string(b)})
		if _, err := s.DB.Exec(`UPDATE documents SET index_complete=? WHERE id=?`, i+1, documentID); err != nil {
			return nil, err
		}
	}
	return prepared, nil
}

func (s *Service) ReindexWorkspace(ctx context.Context, workspaceID int64) error {
	documents, err := s.documentsForReindex(`workspace_id=?`, workspaceID)
	if err != nil {
		return err
	}
	for _, document := range documents {
		if err := s.reindexDocument(ctx, document); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ReindexStale(ctx context.Context) (int, error) {
	documents, err := s.documentsForReindex(`index_version<? OR status<>'ready'`, CurrentIndexVersion)
	if err != nil {
		return 0, err
	}
	for i, document := range documents {
		if err := s.reindexDocument(ctx, document); err != nil {
			return i, err
		}
	}
	return len(documents), nil
}

type storedDocument struct {
	id          int64
	workspaceID int64
	name        string
	path        string
}

func (s *Service) documentsForReindex(where string, args ...any) ([]storedDocument, error) {
	rows, err := s.DB.Query(`SELECT id,workspace_id,name,path FROM documents WHERE `+where+` ORDER BY id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var documents []storedDocument
	for rows.Next() {
		var document storedDocument
		if err := rows.Scan(&document.id, &document.workspaceID, &document.name, &document.path); err != nil {
			return nil, err
		}
		documents = append(documents, document)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return documents, nil
}

func (s *Service) reindexDocument(ctx context.Context, document storedDocument) error {
	if _, err := s.DB.Exec(`UPDATE documents SET status='preparing',index_total=0,index_complete=0 WHERE id=?`, document.id); err != nil {
		return err
	}
	size, overlap := s.chunkSettings(document.workspaceID)
	chunks, err := extractConfigured(document.path, size, overlap)
	if err != nil {
		_, _ = s.DB.Exec(`UPDATE documents SET status='error' WHERE id=?`, document.id)
		return fmt.Errorf("%s: %w", document.name, err)
	}
	if _, err := s.DB.Exec(`UPDATE documents SET status='indexing',index_total=?,index_complete=0 WHERE id=?`, len(chunks), document.id); err != nil {
		return err
	}
	prepared, err := s.prepareIndex(ctx, document.id, document.name, chunks)
	if err != nil {
		_, _ = s.DB.Exec(`UPDATE documents SET status='error' WHERE id=?`, document.id)
		return err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM document_chunks WHERE document_id=?`, document.id); err == nil {
		for index, chunk := range prepared {
			if _, err = tx.Exec(`INSERT INTO document_chunks(document_id,workspace_id,chunk_index,content,source_name,page_number,embedding)
				VALUES(?,?,?,?,?,?,?)`, document.id, document.workspaceID, index, chunk.Content, document.name, chunk.PageNumber, chunk.Embedding); err != nil {
				break
			}
		}
	}
	if err == nil {
		_, err = tx.Exec(`UPDATE documents SET status='ready',index_version=?,index_complete=index_total WHERE id=?`, CurrentIndexVersion, document.id)
	}
	if err != nil {
		tx.Rollback()
		_, _ = s.DB.Exec(`UPDATE documents SET status='error' WHERE id=?`, document.id)
		return err
	}
	return tx.Commit()
}

func extract(path string) ([]extractedChunk, error) {
	return extractConfigured(path, 1000, 200)
}

func extractConfigured(path string, size, overlap int) ([]extractedChunk, error) {
	extension := strings.ToLower(filepath.Ext(path))
	if extension == ".pdf" {
		return extractPDFConfigured(path, size, overlap)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := string(data)
	if extension == ".html" || extension == ".htm" {
		text = extractHTMLText(text)
	}
	return chunksForTextConfigured(text, nil, size, overlap)
}

func extractHTMLText(source string) string {
	source = htmlIgnoredContent.ReplaceAllString(source, " ")
	source = htmlComments.ReplaceAllString(source, " ")
	source = htmlTags.ReplaceAllString(source, " ")
	return html.UnescapeString(source)
}

func extractPDF(path string) ([]extractedChunk, error) {
	return extractPDFConfigured(path, 1000, 200)
}

func extractPDFConfigured(path string, size, overlap int) ([]extractedChunk, error) {
	file, reader, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not read PDF: %w", err)
	}
	defer file.Close()
	var chunks []extractedChunk
	for pageNumber := 1; pageNumber <= reader.NumPage(); pageNumber++ {
		page := reader.Page(pageNumber)
		if page.V.IsNull() || page.V.Key("Contents").Kind() == pdf.Null {
			continue
		}
		text, err := extractPageText(page)
		if err != nil {
			return nil, fmt.Errorf("could not extract page %d: %w", pageNumber, err)
		}
		pageChunks, err := chunksForTextConfigured(text, &pageNumber, size, overlap)
		if err == nil {
			chunks = append(chunks, pageChunks...)
		}
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("this PDF contains no extractable text; it may be scanned or image-only")
	}
	return chunks, nil
}

func extractPageText(page pdf.Page) (string, error) {
	rows, err := page.GetTextByRow()
	if err == nil && len(rows) > 0 {
		var pageText strings.Builder
		for _, row := range rows {
			var parts []string
			for _, text := range row.Content {
				if part := strings.TrimSpace(text.S); part != "" {
					parts = append(parts, part)
				}
			}
			if len(parts) > 0 {
				pageText.WriteString(strings.Join(parts, " "))
				pageText.WriteByte('\n')
			}
		}
		if strings.TrimSpace(pageText.String()) != "" {
			return pageText.String(), nil
		}
	}
	return page.GetPlainText(nil)
}

func chunksForText(text string, pageNumber *int) ([]extractedChunk, error) {
	return chunksForTextConfigured(text, pageNumber, 1000, 200)
}

func chunksForTextConfigured(text string, pageNumber *int, size, overlap int) ([]extractedChunk, error) {
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return nil, fmt.Errorf("document contains no extractable text")
	}
	raw := Chunk(text, size, overlap)
	chunks := make([]extractedChunk, 0, len(raw))
	for _, content := range raw {
		chunks = append(chunks, extractedChunk{Content: content, PageNumber: pageNumber})
	}
	return chunks, nil
}

func (s *Service) chunkSettings(workspaceID int64) (int, int) {
	var profile string
	if err := s.DB.QueryRow(`SELECT index_profile FROM workspaces WHERE id=?`, workspaceID).Scan(&profile); err != nil {
		return 1000, 200
	}
	switch profile {
	case "focused":
		return 700, 120
	case "broad":
		return 1400, 250
	default:
		return 1000, 200
	}
}

func Chunk(text string, size, overlap int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{"No extractable text found."}
	}
	var out []string
	for start := 0; start < len(text); {
		end := start + size
		if end > len(text) {
			end = len(text)
		}
		if end < len(text) {
			if cut := strings.LastIndexAny(text[start:end], ".\n "); cut > size/2 {
				end = start + cut + 1
			}
		}
		out = append(out, strings.TrimSpace(text[start:end]))
		if end == len(text) {
			break
		}
		start = end - overlap
		if start < 0 {
			start = 0
		}
	}
	return out
}
