package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature *float64  `json:"temperature,omitempty"`
	TopP        *float64  `json:"top_p,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
}

type ChatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

type Client struct {
	baseURL        string
	apiKey         string
	httpClient     *http.Client
	maxRetries     int
	initialBackoff time.Duration
}

type ClientOption func(*Client)

func WithMaxRetries(n int) ClientOption {
	return func(c *Client) {
		c.maxRetries = n
	}
}

func WithInitialBackoff(d time.Duration) ClientOption {
	return func(c *Client) {
		c.initialBackoff = d
	}
}

func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

func NewClient(baseURL, apiKey string, opts ...ClientOption) *Client {
	baseURL = strings.TrimSpace(baseURL)
	baseURL = strings.TrimRight(baseURL, "/")

	client := &Client{
		baseURL: baseURL,
		apiKey:  strings.TrimSpace(apiKey),
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: 15 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout:   15 * time.Second,
				ResponseHeaderTimeout: 60 * time.Second,
				IdleConnTimeout:       90 * time.Second,
			},
		},
		maxRetries:     3,
		initialBackoff: 2 * time.Second,
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

func (c *Client) ChatCompletionsURL() string {
	if strings.HasSuffix(c.baseURL, "/chat/completions") {
		return c.baseURL
	}
	return c.baseURL + "/chat/completions"
}

func (c *Client) ModelsURL() string {
	if strings.HasSuffix(c.baseURL, "/models") {
		return c.baseURL
	}
	return c.baseURL + "/models"
}

func isRetryableError(err error, resp *http.Response) bool {
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return true
		}
		if strings.Contains(err.Error(), "timeout") {
			return true
		}
		if strings.Contains(err.Error(), "connection reset") {
			return true
		}
		if strings.Contains(err.Error(), "connection refused") {
			return true
		}
		if strings.Contains(err.Error(), "TLS handshake timeout") {
			return true
		}
	}
	if resp != nil {
		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			return true
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return true
		}
	}
	return false
}

func (c *Client) CreateChatCompletion(ctx context.Context, req ChatCompletionRequest) (string, error) {
	bs, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	url := c.ChatCompletionsURL()
	log.Printf("AI request: POST %s (model=%s, prompt_len=%d, max_retries=%d)", url, req.Model, len(bs), c.maxRetries)

	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := c.initialBackoff * time.Duration(1<<(attempt-1))
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			log.Printf("AI request retry %d/%d, waiting %v before retry", attempt, c.maxRetries, backoff)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bs))
		if err != nil {
			lastErr = fmt.Errorf("create request error: %v", err)
			continue
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "application/json")
		if c.apiKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		}

		start := time.Now()
		resp, err := c.httpClient.Do(httpReq)

		if err != nil {
			lastErr = err
			log.Printf("AI request attempt %d/%d failed after %v: %v", attempt+1, c.maxRetries+1, time.Since(start), err)
			if isRetryableError(err, nil) && attempt < c.maxRetries {
				continue
			}
			return "", fmt.Errorf("AI service error: %v (check network or base_url)", err)
		}

		log.Printf("AI response attempt %d/%d: status=%d (took %v)", attempt+1, c.maxRetries+1, resp.StatusCode, time.Since(start))

		if isRetryableError(nil, resp) && attempt < c.maxRetries {
			resp.Body.Close()
			continue
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read response error: %v", err)
			continue
		}

		var out ChatCompletionResponse
		if err := json.Unmarshal(body, &out); err != nil {
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				snippet := string(body)
				if len(snippet) > 200 {
					snippet = snippet[:200] + "..."
				}
				return "", fmt.Errorf("AI service error: HTTP %d - %s", resp.StatusCode, snippet)
			}
			return "", fmt.Errorf("parse response error: %v", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			if out.Error != nil && strings.TrimSpace(out.Error.Message) != "" {
				return "", fmt.Errorf("AI service error: %s", out.Error.Message)
			}
			snippet := string(body)
			if len(snippet) > 200 {
				snippet = snippet[:200] + "..."
			}
			return "", fmt.Errorf("AI service error: HTTP %d - %s", resp.StatusCode, snippet)
		}

		if out.Error != nil && strings.TrimSpace(out.Error.Message) != "" {
			return "", fmt.Errorf("AI service error: %s", out.Error.Message)
		}

		if len(out.Choices) == 0 {
			return "", fmt.Errorf("AI service error: empty choices")
		}

		content := strings.TrimSpace(out.Choices[0].Message.Content)
		if content == "" {
			return "", fmt.Errorf("AI service error: empty content")
		}

		log.Printf("AI request success: content_length=%d", len(content))
		return content, nil
	}

	if lastErr != nil {
		return "", fmt.Errorf("AI service error after %d retries: %v (check network or base_url)", c.maxRetries+1, lastErr)
	}
	return "", fmt.Errorf("AI service error after %d retries", c.maxRetries+1)
}

type ModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	url := c.ModelsURL()
	log.Printf("AI list models: GET %s (max_retries=%d)", url, c.maxRetries)

	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := c.initialBackoff * time.Duration(1<<(attempt-1))
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			log.Printf("AI list models retry %d/%d, waiting %v before retry", attempt, c.maxRetries, backoff)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			lastErr = fmt.Errorf("create request error: %v", err)
			continue
		}
		httpReq.Header.Set("Accept", "application/json")
		if c.apiKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		}

		start := time.Now()
		resp, err := c.httpClient.Do(httpReq)

		if err != nil {
			lastErr = err
			log.Printf("AI list models attempt %d/%d failed after %v: %v", attempt+1, c.maxRetries+1, time.Since(start), err)
			if isRetryableError(err, nil) && attempt < c.maxRetries {
				continue
			}
			return nil, fmt.Errorf("AI service error: %v (check network or base_url)", err)
		}

		log.Printf("AI list models response attempt %d/%d: status=%d (took %v)", attempt+1, c.maxRetries+1, resp.StatusCode, time.Since(start))

		if isRetryableError(nil, resp) && attempt < c.maxRetries {
			resp.Body.Close()
			continue
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read response error: %v", err)
			continue
		}

		var out ModelsResponse
		if err := json.Unmarshal(body, &out); err != nil {
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				snippet := string(body)
				if len(snippet) > 200 {
					snippet = snippet[:200] + "..."
				}
				return nil, fmt.Errorf("AI service error: HTTP %d - %s", resp.StatusCode, snippet)
			}
			return nil, fmt.Errorf("parse response error: %v", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			if out.Error != nil && strings.TrimSpace(out.Error.Message) != "" {
				return nil, fmt.Errorf("AI service error: %s", out.Error.Message)
			}
			snippet := string(body)
			if len(snippet) > 200 {
				snippet = snippet[:200] + "..."
			}
			return nil, fmt.Errorf("AI service error: HTTP %d - %s", resp.StatusCode, snippet)
		}

		if out.Error != nil && strings.TrimSpace(out.Error.Message) != "" {
			return nil, fmt.Errorf("AI service error: %s", out.Error.Message)
		}

		ids := make([]string, 0, len(out.Data))
		for _, m := range out.Data {
			id := strings.TrimSpace(m.ID)
			if id != "" {
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			return nil, fmt.Errorf("AI service error: empty models list")
		}

		log.Printf("AI list models success: %d models found", len(ids))
		return ids, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("AI service error after %d retries: %v (check network or base_url)", c.maxRetries+1, lastErr)
	}
	return nil, fmt.Errorf("AI service error after %d retries", c.maxRetries+1)
}
