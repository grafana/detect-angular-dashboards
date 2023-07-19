package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

var ErrBadStatusCode = fmt.Errorf("bad status code")

type Client struct {
	BaseURL string
	Token   string
}

func NewClient(baseURL string, token string) Client {
	return Client{
		BaseURL: baseURL,
		Token:   token,
	}
}

func (cl Client) urlFor(s string) string {
	return cl.BaseURL + "/" + s
}

func (cl Client) newRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cl.urlFor(url), nil)
	if err != nil {
		return nil, err
	}
	if cl.Token != "" {
		req.Header.Add("Authorization", "Bearer "+cl.Token)
	}
	return req, err
}

func (cl Client) Request(ctx context.Context, url string, out interface{}) error {
	req, err := cl.newRequest(ctx, url)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %d", ErrBadStatusCode, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}
