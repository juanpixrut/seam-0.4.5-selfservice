package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CLIXOLUClient is a simple XOLU client for the CLI.
type CLIXOLUClient struct {
	baseURL string
	client  *http.Client
}

func (c *CLIXOLUClient) httpClient() *http.Client {
	if c.client == nil {
		c.client = &http.Client{Timeout: 30 * time.Second}
	}
	return c.client
}

// Create creates a new entity and returns its ID.
func (c *CLIXOLUClient) Create(ctx context.Context, entityType string, data map[string]interface{}) (int64, error) {
	url := fmt.Sprintf("%s/api/v1/%s", c.baseURL, entityType)

	body, err := json.Marshal(data)
	if err != nil {
		return 0, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return 0, fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode: %w", err)
	}

	return result.ID, nil
}

// Update updates an entity.
func (c *CLIXOLUClient) Update(ctx context.Context, entityType string, id int64, data map[string]interface{}) error {
	url := fmt.Sprintf("%s/api/v1/%s/%d", c.baseURL, entityType, id)

	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// Delete deletes an entity.
func (c *CLIXOLUClient) Delete(ctx context.Context, entityType string, id int64) error {
	url := fmt.Sprintf("%s/api/v1/%s/%d", c.baseURL, entityType, id)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// OQL executes an OQL query.
func (c *CLIXOLUClient) OQL(ctx context.Context, query string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1/oql/query", c.baseURL)

	body, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	// xolu OQL response envelope: {"data": [...], "stats": {...}}
	var result struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	return result.Data, nil
}

// GetByEmail gets an entity by email.
func (c *CLIXOLUClient) GetByEmail(ctx context.Context, entityType, email string) (map[string]interface{}, error) {
	query := fmt.Sprintf("SELECT TOP 1 * FROM %s WHERE email = '%s'", entityType, escapeString(email))
	rows, err := c.OQL(ctx, query)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("not found")
	}
	return rows[0], nil
}

func escapeString(s string) string {
	// Simple escape for single quotes
	result := make([]byte, 0, len(s)*2)
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			result = append(result, '\'', '\'')
		} else {
			result = append(result, s[i])
		}
	}
	return string(result)
}
