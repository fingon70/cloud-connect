package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
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
	Chash string `json:"chash"`
	Mhash string `json:"mhash"`
	Nhash string `json:"nhash"`
}

func (c *Client) ListDir(ctx context.Context, path string) ([]model.Entry, error) {
	query := url.Values{}
	query.Set("path", normalizePath(path))
	query.Set("fields", "members.name,members.path,members.type,members.size,members.mtime,members.chash,members.mhash,members.nhash")

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
			Chash:     member.Chash,
			Mhash:     member.Mhash,
			Nhash:     member.Nhash,
		})
	}

	return entries, nil
}

func (c *Client) CreateDir(ctx context.Context, path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is required")
	}
	query := url.Values{}
	query.Set("path", normalizePath(path))
	req, err := c.newRequest(ctx, http.MethodPost, "/dir", query)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

func (c *Client) EnsureDir(ctx context.Context, dir string) error {
	dir = normalizePath(strings.TrimSpace(dir))
	if dir == "" || dir == "/" {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(dir, "/"), "/")
	current := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		current = current + "/" + part
		entry, err := c.GetMeta(ctx, current)
		if err == nil {
			if entry.Type != model.EntryFolder {
				return fmt.Errorf("remote path is not a directory: %s", current)
			}
			continue
		}
		if IsNotFound(err) {
			if err := c.CreateDir(ctx, current); err != nil {
				if IsConflict(err) {
					continue
				}
				return err
			}
			continue
		}
		return err
	}
	return nil
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
