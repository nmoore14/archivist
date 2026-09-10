package rag

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"archivist/internal/models"
)

//go:embed prompts/system.md
var systemPrompt string

var (
	generatedReferenceHeading = regexp.MustCompile(`(?im)^\s{0,3}(?:#{1,6}\s*)?(?:source references?|references|bibliography|sources)\s*:?\s*$`)
	markdownWebLink           = regexp.MustCompile(`\[([^\]]+)\]\(https?://[^)]+\)`)
	bareWebURL                = regexp.MustCompile(`https?://[^\s<>)]+`)
	questionWebURL            = regexp.MustCompile(`(?i)\b(?:https?://|www\.)[^\s<>()]+`)
)

type Service struct {
	DB                                        *sql.DB
	Ollama                                    *models.Client
	ChatModel, EmbedModel                     string
	SystemPrompt, ContextLabel, QuestionLabel string
	DefaultRetrievalCount                     int
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
	if questionWebURL.MatchString(question) {
		return "Archivist cannot open or retrieve web addresses. Please ask about content already stored in this private library.", nil, nil
	}
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
	retrievalCount := s.DefaultRetrievalCount
	if retrievalCount == 0 {
		retrievalCount = 4
	}
	_ = s.DB.QueryRow(`SELECT retrieval_count FROM workspaces WHERE id=?`, workspaceID).Scan(&retrievalCount)
	if retrievalCount != 3 && retrievalCount != 4 && retrievalCount != 6 {
		retrievalCount = 4
	}
	if len(hits) > retrievalCount {
		hits = hits[:retrievalCount]
	}
	if len(hits) == 0 {
		return "The available documents do not provide enough information to answer that yet. Ask an administrator to add or reindex trusted content.", nil, nil
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
	contextLabel, questionLabel := s.ContextLabel, s.QuestionLabel
	if contextLabel == "" {
		contextLabel = "Course context"
	}
	if questionLabel == "" {
		questionLabel = "Student question"
	}
	prompt := s.SystemPrompt
	if strings.TrimSpace(prompt) == "" {
		prompt = systemPrompt
	}
	userPrompt := contextLabel + ":\n" + contextText.String() + "\n\n" + questionLabel + ":\n" + question
	messages := []models.ChatMessage{
		{Role: "system", Content: strings.TrimSpace(prompt)},
		{Role: "user", Content: userPrompt},
	}
	answer, err := s.Ollama.Chat(ctx, s.ChatModel, messages)
	return enforceLocalAnswerWithContext(answer, contextText.String()), sources, err
}

func enforceLocalAnswer(answer string) string {
	return enforceLocalAnswerWithContext(answer, "")
}

func enforceLocalAnswerWithContext(answer, courseContext string) string {
	if location := generatedReferenceHeading.FindStringIndex(answer); location != nil {
		answer = answer[:location[0]]
	}
	allowedURLs := map[string]bool{}
	for _, url := range bareWebURL.FindAllString(courseContext, -1) {
		allowedURLs[url] = true
	}
	answer = markdownWebLink.ReplaceAllStringFunc(answer, func(link string) string {
		parts := markdownWebLink.FindStringSubmatch(link)
		if len(parts) != 2 {
			return link
		}
		url := bareWebURL.FindString(link)
		if !allowedURLs[url] {
			return parts[1]
		}
		return link
	})
	answer = bareWebURL.ReplaceAllStringFunc(answer, func(url string) string {
		if allowedURLs[url] {
			return url
		}
		return ""
	})
	return strings.TrimSpace(answer)
}
