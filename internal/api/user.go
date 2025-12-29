package api

import (
	"context"
	"net/http"
	"net/url"
)

type userResponse struct {
	Alias string `json:"alias"`
	Home  string `json:"home"`
}

func (c *Client) GetUser(ctx context.Context) (userResponse, error) {
	query := url.Values{}
	query.Set("fields", "alias,home")

	req, err := c.newRequest(ctx, http.MethodGet, "/user", query)
	if err != nil {
		return userResponse{}, err
	}

	var resp []userResponse
	if err := c.do(req, &resp); err != nil {
		return userResponse{}, err
	}
	if len(resp) == 0 {
		return userResponse{}, nil
	}
	return resp[0], nil
}
