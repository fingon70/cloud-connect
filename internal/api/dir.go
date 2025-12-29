package api

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/fingon70/cloud-connect/internal/model"
)

type dirResponse struct {
	Members []dirMember `json:"members"`
}

type dirMember struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Type  string `json:"type"`
	Size  int64  `json:"size"`
	Mtime int64  `json:"mtime"`
}

func (c *Client) ListDir(ctx context.Context, path string) ([]model.Entry, error) {
	query := url.Values{}
	query.Set("path", path)
	query.Set("fields", "members.name,members.path,members.type,members.size,members.mtime")

	req, err := c.newRequest(ctx, http.MethodGet, "/dir", query)
	if err != nil {
		return nil, err
	}

	var resp dirResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	entries := make([]model.Entry, 0, len(resp.Members))
	for _, member := range resp.Members {
		entryType := model.EntryFile
		if member.Type == "dir" {
			entryType = model.EntryFolder
		}
		name := decodePath(member.Name)
		memberPath := decodePath(member.Path)
		entries = append(entries, model.Entry{
			Name:      name,
			Path:      memberPath,
			Type:      entryType,
			Size:      member.Size,
			UpdatedAt: time.Unix(member.Mtime, 0),
		})
	}

	return entries, nil
}

func decodePath(value string) string {
	if value == "" {
		return value
	}
	decoded, err := url.PathUnescape(value)
	if err != nil {
		return value
	}
	return decoded
}
