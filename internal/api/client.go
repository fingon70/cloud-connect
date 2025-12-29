package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	Token      string
}

type APIError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *APIError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("hidrive api error: %s", e.Status)
	}
	return fmt.Sprintf("hidrive api error: %s: %s", e.Status, e.Body)
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		Token:      token,
	}
}

func DefaultBaseURL() string {
	return "https://api.hidrive.strato.com/2.1"
}

func (c *Client) newRequest(ctx context.Context, method, path string, query url.Values) (*http.Request, error) {
	base := strings.TrimRight(c.BaseURL, "/")
	endpoint := fmt.Sprintf("%s%s", base, path)
	if len(query) > 0 {
		endpoint = endpoint + "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiError(resp)
	}

	if out == nil {
		return nil
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) doStream(req *http.Request, out io.Writer) error {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiError(resp)
	}

	_, err = io.Copy(out, resp.Body)
	return err
}

func apiError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	return &APIError{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Body:       strings.TrimSpace(string(body)),
	}
}
