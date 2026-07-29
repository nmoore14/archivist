package models

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
}
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type ModelInfo struct {
	Name string `json:"name"`
}

func New(base string) *Client {
	return &Client{BaseURL: strings.TrimRight(base, "/"), HTTP: &http.Client{Timeout: 2 * time.Minute}}
}
func (c *Client) post(ctx context.Context, path string, in, out any) error {
	b, _ := json.Marshal(in)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("ollama: %s", res.Status)
	}
	return json.NewDecoder(res.Body).Decode(out)
}
func (c *Client) GenerateEmbedding(ctx context.Context, model, text string) ([]float64, error) {
	var out struct {
		Embedding []float64 `json:"embedding"`
	}
	err := c.post(ctx, "/api/embeddings", map[string]any{"model": model, "prompt": text}, &out)
	return out.Embedding, err
}
func (c *Client) Chat(ctx context.Context, model string, messages []ChatMessage) (string, error) {
	var out struct {
		Message ChatMessage `json:"message"`
	}
	err := c.post(ctx, "/api/chat", map[string]any{"model": model, "messages": messages, "stream": false}, &out)
	return out.Message.Content, err
}
func (c *Client) ListModels(ctx context.Context) ([]ModelInfo, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/tags", nil)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var out struct {
		Models []ModelInfo `json:"models"`
	}
	err = json.NewDecoder(res.Body).Decode(&out)
	return out.Models, err
}
func (c *Client) Health(ctx context.Context) error { _, err := c.ListModels(ctx); return err }
