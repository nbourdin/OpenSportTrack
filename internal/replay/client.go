package replay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"opensporttrack/internal/tracking"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), HTTP: &http.Client{Timeout: 15 * time.Second}}
}

func (c *Client) request(ctx context.Context, method string, path string, payload any, response any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, strings.TrimSpace(string(message)))
	}
	if response != nil {
		if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

func (c *Client) CreateActivity(ctx context.Context) (tracking.Activity, error) {
	var activity tracking.Activity
	err := c.request(ctx, http.MethodPost, "/api/v1/activities", map[string]string{"sport": "running"}, &activity)
	return activity, err
}

func (c *Client) SendSample(ctx context.Context, id string, sample tracking.Sample) error {
	return c.request(ctx, http.MethodPost, "/api/v1/activities/"+id+"/samples", sample, nil)
}

func (c *Client) SetRoute(ctx context.Context, id string, positions []tracking.Position) error {
	return c.request(ctx, http.MethodPut, "/api/v1/activities/"+id+"/route", struct {
		Positions []tracking.Position `json:"positions"`
	}{Positions: positions}, nil)
}
