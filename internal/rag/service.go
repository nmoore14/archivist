package rag

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"archivist/internal/models"
)

var (
	generatedReferenceHeading = regexp.MustCompile(`(?im)^\s{0,3}(?:#{1,6}\s*)?(?:source references?|references|bibliography|sources)\s*:?\s*$`)
	markdownWebLink           = regexp.MustCompile(`\[([^\]]+)\]\(https?://[^)]+\)`)
	bareWebURL                = regexp.MustCompile(`https?://[^\s<>)]+`)
)

type Service struct {
	DB                    *sql.DB
	Ollama                *models.Client
	ChatModel, EmbedModel string
}
type hit struct {
	content, source string
	score           float64
}

func cosine(a, b []float64) float64 {
	var dot, aa, bb float64
	for i := 0; i < len(a) && i < len(b); i++ {
		dot += a[i] * b[i]
		aa += a[i] * a[i]
		bb += b[i] * b[i]
	}
	if aa == 0 || bb == 0 {
		return 0
	}
	return dot / (math.Sqrt(aa) * math.Sqrt(bb))
}
func (s *Service) Answer(ctx context.Context, workspaceID int64, question string) (string, []string, error) {
	qv, err := s.Ollama.GenerateEmbedding(ctx, s.EmbedModel, question)
	if err != nil {
		return "", nil, fmt.Errorf("local model unavailable: %w", err)
	}
	rows, err := s.DB.Query(`SELECT content,
		CASE WHEN page_number IS NULL THEN source_name ELSE source_name || ', page ' || page_number END,
		embedding FROM document_chunks WHERE workspace_id=? AND embedding<>''`, workspaceID)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()
	var hits []hit
	for rows.Next() {
		var content, source, raw string
		if rows.Scan(&content, &source, &raw) != nil {
			continue
		}
		var v []float64
		if json.Unmarshal([]byte(raw), &v) == nil {
			hits = append(hits, hit{content, source, cosine(qv, v)})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	retrievalCount := 4
	_ = s.DB.QueryRow(`SELECT retrieval_count FROM workspaces WHERE id=?`, workspaceID).Scan(&retrievalCount)
	if retrievalCount != 3 && retrievalCount != 4 && retrievalCount != 6 {
		retrievalCount = 4
	}
	if len(hits) > retrievalCount {
		hits = hits[:retrievalCount]
	}
	if len(hits) == 0 {
		return "The course materials do not provide enough information to answer that yet. Ask an administrator to add or reindex course documents.", nil, nil
	}
	var contextText strings.Builder
	seen := map[string]bool{}
	var sources []string
	for _, h := range hits {
		fmt.Fprintf(&contextText, "\n[%s]\n%s\n", h.source, h.content)
		if !seen[h.source] {
			seen[h.source] = true
			sources = append(sources, h.source)
		}
	}
	prompt := `You are Archivist, a course assistant running entirely on this local server.

Answer using only facts and instructions explicitly present in the course context below. Do not fill gaps with general knowledge, training memory, or plausible examples. If the requested detail is not in the context, say that the course materials do not provide enough information.

Do not write a Sources, References, Source references, or Bibliography section. Do not include web links or recommend external documentation. The application displays its own verified source citations beneath your answer.

Use Markdown when it makes the answer easier to read. When an answer contains mathematics, write valid LaTeX. Delimit inline formulas with \( and \), and display formulas with \[ and \]. Do not place LaTeX inside code fences.` + "\n\nCourse context:\n" + contextText.String() + "\n\nStudent question:\n" + question
	answer, err := s.Ollama.Chat(ctx, s.ChatModel, []models.ChatMessage{{Role: "user", Content: prompt}})
	return enforceLocalAnswer(answer), sources, err
}

func enforceLocalAnswer(answer string) string {
	if location := generatedReferenceHeading.FindStringIndex(answer); location != nil {
		answer = answer[:location[0]]
	}
	answer = markdownWebLink.ReplaceAllString(answer, "$1")
	answer = bareWebURL.ReplaceAllString(answer, "")
	return strings.TrimSpace(answer)
}
