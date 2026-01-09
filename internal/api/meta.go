package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/fingon70/cloud-connect/internal/model"
)

type metaResponse struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Type  string `json:"type"`
	Size  int64  `json:"size"`
	Mtime int64  `json:"mtime"`
	Chash string `json:"chash"`
	Mhash string `json:"mhash"`
	Nhash string `json:"nhash"`
}

func (c *Client) GetMeta(ctx context.Context, path string) (model.Entry, error) {
	query := url.Values{}
	query.Set("path", normalizePath(path))
	query.Set("fields", "name,path,type,size,mtime,chash,mhash,nhash")

	req, err := c.newRequest(ctx, http.MethodGet, "/meta", query)
	if err != nil {
		return model.Entry{}, err
	}

	var raw json.RawMessage
	if err := c.do(req, &raw); err != nil {
		return model.Entry{}, err
	}

	meta, err := parseMeta(raw)
	if err != nil {
		return model.Entry{}, err
	}

	entryType := model.EntryFile
	if meta.Type == "dir" {
		entryType = model.EntryFolder
	}

	return model.Entry{
		Name:      decodePath(meta.Name),
		Path:      decodePath(meta.Path),
		Type:      entryType,
		Size:      meta.Size,
		UpdatedAt: time.Unix(meta.Mtime, 0),
		Chash:     meta.Chash,
		Mhash:     meta.Mhash,
		Nhash:     meta.Nhash,
	}, nil
}

func parseMeta(raw json.RawMessage) (metaResponse, error) {
	var meta metaResponse
	if err := json.Unmarshal(raw, &meta); err == nil {
		if meta.Path != "" || meta.Name != "" || meta.Type != "" {
			return meta, nil
		}
	}
	var list []metaResponse
	if err := json.Unmarshal(raw, &list); err == nil && len(list) > 0 {
		return list[0], nil
	}
	return metaResponse{}, errors.New("unexpected meta response")
}
