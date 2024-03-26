package gcom

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/grafana/detect-angular-dashboards/api"
)

type APIClient struct {
	api.Client
}

func NewAPIClient() APIClient {
	return APIClient{
		Client: api.NewClient("https://grafana.com/api"),
	}
}

func (cl APIClient) GetAngularDetected(ctx context.Context, slug, version string) (bool, error) {
	var resp PluginVersions
	if err := cl.Request(ctx, http.MethodGet, "plugins/"+slug+"/versions", &resp); err != nil {
		if errors.Is(err, api.ErrBadStatusCode) {
			// Swallow bad status codes
			return false, nil
		}
		return false, fmt.Errorf("request: %w", err)
	}
	for _, pv := range resp.Items {
		if pv.Version == version {
			return pv.AngularDetected, nil
		}
	}
	return false, nil
}
