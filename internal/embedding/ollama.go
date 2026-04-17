package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var ErrDisabled = errors.New("embedding disabled")

type Client struct {
	host  string
	model string
	http  *http.Client
}

func NewClient(host, model string) *Client {
	return &Client{
		host:  strings.TrimRight(host, "/"),
		model: model,
		http:  &http.Client{},
	}
}

func (c *Client) Enabled() bool {
	return c.host != "" && c.model != ""
}

func (c *Client) Model() string {
	return c.model
}

func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	if !c.Enabled() {
		return nil, ErrDisabled
	}

	reqBody, err := json.Marshal(map[string]string{
		"model":  c.model,
		"prompt": text,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.host+"/api/embeddings", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("build embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embed returned %d", resp.StatusCode)
	}

	var out struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}
	if len(out.Embedding) == 0 {
		return nil, errors.New("embed returned empty vector")
	}
	return out.Embedding, nil
}
