package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (c *Client) DownloadFile(ctx context.Context, path string, out io.Writer) error {
	query := url.Values{}
	query.Set("path", normalizePath(path))

	req, err := c.newRequest(ctx, http.MethodGet, "/file", query)
	if err != nil {
		return err
	}

	return c.doStream(req, out)
}

func (c *Client) UploadFile(ctx context.Context, dir, name string, content io.Reader, contentType string, overwrite bool, mtime time.Time) error {
	if strings.TrimSpace(dir) == "" {
		return errors.New("dir is required")
	}
	if strings.TrimSpace(name) == "" {
		return errors.New("name is required")
	}
	query := url.Values{}
	query.Set("dir", normalizePath(dir))
	query.Set("name", name)
	if !mtime.IsZero() {
		query.Set("mtime", fmt.Sprintf("%d", mtime.Unix()))
	}
	method := http.MethodPost
	if overwrite {
		method = http.MethodPut
	}
	req, err := c.newRequestWithBody(ctx, method, "/file", query, content)
	if err != nil {
		return err
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	req.Header.Set("Content-Type", contentType)
	return c.do(req, nil)
}
