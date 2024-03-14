package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

var ErrBadStatusCode = fmt.Errorf("bad status code")

type Client struct {
	BaseURL           string
	Token             string
	BasicAuthUser     string
	BasicAuthPassword string
}

func NewClient(baseURL string, token string) Client {
	auth := strings.SplitN(token, ":", 2)
	if len(auth) == 2 {
		return Client{
			BaseURL:           baseURL,
			BasicAuthUser:     auth[0],
			BasicAuthPassword: auth[1],
		}
	}
	return Client{
		BaseURL: baseURL,
		Token:   auth[0],
	}
}

func (cl Client) urlFor(s string) string {
	return cl.BaseURL + "/" + s
}

func (cl Client) newRequest(ctx context.Context, method, url string) (*http.Request, error) {
	if method == "" {
		method = http.MethodGet
	}
	req, err := http.NewRequestWithContext(ctx, method, cl.urlFor(url), nil)
	if err != nil {
		return nil, err
	}
	// There is two cases, either we have provided a service account's Token or
	// the basicAuth. As the token is the recommended way to interact with the
	// API let's use it first
	if cl.Token != "" {
		req.Header.Add("Authorization", "Bearer "+cl.Token)
	} else if cl.BasicAuthUser != "" && cl.BasicAuthPassword != "" {
		req.SetBasicAuth(cl.BasicAuthUser, cl.BasicAuthPassword)
	}
	return req, err
}

func (cl Client) Request(ctx context.Context, method, url string, out interface{}) error {
	req, err := cl.newRequest(ctx, method, url)
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
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode: %w", err)
		}
	}
	return nil
}
