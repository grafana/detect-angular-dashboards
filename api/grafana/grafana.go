package grafana

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/grafana/detect-angular-dashboards/api"
)

var errUnknownAngularStatus = errors.New("could not determine if plugin is angular or not, use GCOM instead")

const DefaultBaseURL = "http://127.0.0.1:3000/api"

type APIClient struct {
	api.Client
}

func NewAPIClient(client api.Client) APIClient {
	return APIClient{Client: client}
}

func (cl APIClient) BaseURL() string {
	return cl.Client.BaseURL
}

func (cl APIClient) GetPlugins(ctx context.Context) ([]Plugin, error) {
	var out []Plugin
	err := cl.Request(ctx, http.MethodGet, "plugins", &out)
	return out, err
}

func (cl APIClient) GetDatasourcePluginIDs(ctx context.Context) ([]Datasource, error) {
	var out []Datasource
	err := cl.Request(ctx, http.MethodGet, "datasources", &out)
	return out, err
}

func (cl APIClient) GetDashboards(ctx context.Context, page int) ([]ListedDashboard, error) {
	var out []ListedDashboard
	err := cl.Request(ctx, http.MethodGet, "search?"+url.Values{
		"limit": []string{"5000"},
		"page":  []string{strconv.Itoa(page)},
	}.Encode(), &out)
	return out, err
}

func (cl APIClient) GetDashboard(ctx context.Context, uid string) (*DashboardDefinition, error) {
	var out *DashboardDefinition
	if err := cl.Request(ctx, http.MethodGet, "dashboards/uid/"+uid, &out); err != nil {
		return nil, err
	}
	ConvertPanels(out.Dashboard.Panels)
	return out, nil
}

// ConvertPanels recursively converts datasources map[string]interface{} to custom type.
// The datasource field can either be a string (old) or object (new).
// Could check for schema, but this is easier.
func ConvertPanels(panels []*DashboardPanel) {
	for _, panel := range panels {
		// Recurse
		if len(panel.Panels) > 0 {
			ConvertPanels(panel.Panels)
		}

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
}

func (cl APIClient) GetOrgs(ctx context.Context) ([]Org, error) {
	var orgs []Org
	err := cl.Request(ctx, http.MethodGet, "orgs?"+url.Values{
		"perpage": []string{"1000"},
	}.Encode(), &orgs)
	return orgs, err
}

func (cl APIClient) UserSwitchContext(ctx context.Context, orgID string) error {
	return cl.Request(ctx, http.MethodPost, "user/using/"+orgID, nil)
}

// FrontendSettings is the response returned by api/frontend/settings
type FrontendSettings struct {
	// Panels is a map from panel plugin id to plugin metadata
	Panels map[string]FrontendSettingsPanel

	// Datasources is a map from datasource names to plugin metadata
	Datasources map[string]FrontendSettingsDatasource
}

// FrontendSettingsPanel is a panel present in FrontendSettings.
// Which fields are populated depends on the Grafana version.
type FrontendSettingsPanel struct {
	// AngularDetected is true if the plugin uses Angular APIs
	// (present in Grafana >= 10.1.0 && < 10.3.0)
	AngularDetected *bool

	// Angular contains the Angular metadata for the plugin
	// (present in Grafana >= 10.3.0)
	Angular *struct {
		// Detected is true if the plugin uses Angular APIs
		Detected bool
	}
}

// IsAngular returns true if the panel plugin is an angular plugin.
// The correct fields are used depending on the Grafana version.
// If this information cannot be determined, it returns an errUnknownAngularStatus.
func (p FrontendSettingsPanel) IsAngular() (bool, error) {
	if p.Angular != nil {
		// >= 10.3.0
		return p.Angular.Detected, nil
	}
	if p.AngularDetected != nil {
		// >= 10.1.0 && < 10.3.0
		return *p.AngularDetected, nil
	}
	// < 10.1.0
	return false, errUnknownAngularStatus
}

// FrontendSettingsDatasource is a datasource present in FrontendSettings.
// Which fields are populated depends on the Grafana version.
type FrontendSettingsDatasource struct {
	// AngularDetected is true if the plugin uses Angular APIs
	// (present in Grafana >= 10.1.0 && < 10.3.0)
	AngularDetected *bool

	// Meta contains plugin metadata
	Meta struct {
		// Angular contains angular plugin metadata
		// (present in Grafana >= 10.3.0)
		Angular *struct {
			// Detected is true if the plugin uses Angular APIs
			Detected bool
		}
	}

	// Type is the plugin's ID
	Type string
}

// IsAngular returns true if the datasource plugin is an angular plugin.
// The correct fields are used depending on the Grafana version.
// If this information cannot be determined, it returns an errUnknownAngularStatus.
func (d FrontendSettingsDatasource) IsAngular() (bool, error) {
	if d.Meta.Angular != nil {
		// >= 10.3.0
		return d.Meta.Angular.Detected, nil
	}
	if d.AngularDetected != nil {
		// >= 10.1.0 && < 10.3.0
		return *d.AngularDetected, nil
	}
	// < 10.1.0
	return false, errUnknownAngularStatus
}

func (cl APIClient) GetFrontendSettings(ctx context.Context) (*FrontendSettings, error) {
	var out FrontendSettings
	if err := cl.Request(ctx, http.MethodGet, "frontend/settings", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (cl APIClient) GetServiceAccountPermissions(ctx context.Context) (map[string][]string, error) {
	var out map[string][]string
	if err := cl.Request(ctx, http.MethodGet, "access-control/user/permissions", &out); err != nil {
		return nil, err
	}
	return out, nil
}
