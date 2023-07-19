package grafana

import (
	"context"
	"net/url"
	"strconv"

	"github.com/grafana/detect-angular-dashboards/api"
)

const DefaultBaseURL = "http://127.0.0.1:3000/api"

type APIClient struct {
	api.Client
}

func NewAPIClient(baseURL string, token string) APIClient {
	return APIClient{
		Client: api.NewClient(baseURL, token),
	}
}

func (cl APIClient) GetPlugins(ctx context.Context) ([]Plugin, error) {
	var out []Plugin
	err := cl.Request(ctx, "plugins", &out)
	return out, err
}

func (cl APIClient) GetDatasourcePluginIDs(ctx context.Context) ([]Datasource, error) {
	var out []Datasource
	err := cl.Request(ctx, "datasources", &out)
	return out, err
}

func (cl APIClient) GetDashboards(ctx context.Context, page int) ([]ListedDashboard, error) {
	var out []ListedDashboard
	err := cl.Request(ctx, "search?"+url.Values{
		"limit": []string{"5000"},
		"page":  []string{strconv.Itoa(page)},
	}.Encode(), &out)
	return out, err
}

func (cl APIClient) GetDashboard(ctx context.Context, uid string) (*Dashboard, error) {
	var out struct {
		Dashboard *Dashboard
	}
	if err := cl.Request(ctx, "dashboards/uid/"+uid, &out); err != nil {
		return nil, err
	}
	// Convert datasources map[string]interface{} to custom type
	// The datasource field can either be a string (old) or object (new)
	// Could check for schema, but this is easier
	for _, panel := range out.Dashboard.Panels {
		m, ok := panel.Datasource.(map[string]interface{})
		if !ok {
			// String, keep as-is
			continue
		}
		// Use struct instead of generic map

		// (pointer to value)
		if m["type"] == nil {
			m["type"] = ""
		}
		panel.Datasource = PanelDatasource{Type: m["type"].(string)}
	}

	return out.Dashboard, nil
}
