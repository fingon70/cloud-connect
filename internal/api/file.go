package api

import (
	"context"
	"io"
	"net/http"
	"net/url"
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
