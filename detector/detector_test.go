package detector

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/detect-angular-dashboards/api/gcom"
	"github.com/grafana/detect-angular-dashboards/api/grafana"
	"github.com/grafana/detect-angular-dashboards/logger"
	"github.com/grafana/detect-angular-dashboards/output"
)

func TestDetector(t *testing.T) {
	t.Run("meta", func(t *testing.T) {
		cl := NewTestAPIClient(filepath.Join("testdata", "dashboards", "graph-old.json"))
		d := NewDetector(logger.NewLeveledLogger(false), cl, gcom.NewAPIClient(), 5)
		out, err := d.Run(context.Background())
		require.NoError(t, err)
		require.Len(t, out, 1)
		require.Equal(t, "test case dashboard", out[0].Title)
		require.Equal(t, "test case folder", out[0].Folder)
		require.Equal(t, "d/test-case-dashboard/test-case-dashboard", out[0].URL)
		require.Equal(t, "admin", out[0].CreatedBy)
		require.Equal(t, "admin", out[0].UpdatedBy)
		require.Equal(t, "2023-11-07T11:13:24+01:00", out[0].Created)
		require.Equal(t, "2024-02-21T13:09:27+01:00", out[0].Updated)
	})

	type expDetection struct {
		pluginID      string
		detectionType output.DetectionType
		title         string
		message       string
	}
	for _, tc := range []struct {
		name          string
		file          string
		expDetections []expDetection
	}{
		{
			name: "legacy panel",
			file: "graph-old.json",
			expDetections: []expDetection{{
				pluginID:      "graph",
				detectionType: output.DetectionTypeLegacyPanel,
				title:         "Flot graph",
				message:       `Found legacy plugin "graph" in panel "Flot graph". It can be migrated to a React-based panel by Grafana when opening the dashboard.`,
			}},
		},
		{
			name: "angular panel",
			file: "datatable.json",
			expDetections: []expDetection{{
				pluginID:      "briangann-datatable-panel",
				detectionType: output.DetectionTypePanel,
				title:         "Panel Title",
				message:       `Found angular panel "Panel Title" ("briangann-datatable-panel")`,
			}},
		},
		{
			name: "datasource",
			file: "datasource.json",
			expDetections: []expDetection{{
				pluginID:      "akumuli-datasource",
				detectionType: output.DetectionTypeDatasource,
				title:         "akumuli",
				message:       `Found panel with angular data source "akumuli" ("akumuli-datasource")`,
			}},
		},
		{
			name: "multiple",
			file: "multiple.json",
			expDetections: []expDetection{
				{pluginID: "akumuli-datasource", detectionType: output.DetectionTypeDatasource, title: "akumuli"},
				{pluginID: "briangann-datatable-panel", detectionType: output.DetectionTypePanel, title: "datatable + akumuli"},
				{pluginID: "akumuli-datasource", detectionType: output.DetectionTypeDatasource, title: "datatable + akumuli"},
				{pluginID: "graph", detectionType: output.DetectionTypeLegacyPanel, title: "graph-old"},
			},
		},
		{
			name:          "not angular",
			file:          "not-angular.json",
			expDetections: nil,
		},
		{
			name: "mix of angular and react",
			file: "mixed.json",
			expDetections: []expDetection{
				{pluginID: "briangann-datatable-panel", detectionType: output.DetectionTypePanel, title: "angular"},
			},
		},
		{
			name: "rows expanded",
			file: "rows-expanded.json",
			expDetections: []expDetection{
				{pluginID: "briangann-datatable-panel", detectionType: output.DetectionTypePanel, title: "expanded"},
			},
		},
		{
			name: "rows collapsed",
			file: "rows-collapsed.json",
			expDetections: []expDetection{
				{pluginID: "briangann-datatable-panel", detectionType: output.DetectionTypePanel, title: "collapsed"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cl := NewTestAPIClient(filepath.Join("testdata", "dashboards", tc.file))
			d := NewDetector(logger.NewLeveledLogger(false), cl, gcom.NewAPIClient(), 5)
			out, err := d.Run(context.Background())
			require.NoError(t, err)
			require.Len(t, out, 1, "should have result for one dashboard")
			detections := out[0].Detections
			require.Len(t, detections, len(tc.expDetections), "should have the correct number of detections in the dashboard")
			for i, actual := range detections {
				exp := tc.expDetections[i]
				require.Equal(t, exp.pluginID, actual.PluginID)
				require.Equal(t, exp.detectionType, actual.DetectionType)
				require.Equal(t, exp.title, actual.Title)
				if exp.message != "" {
					require.Equal(t, exp.message, actual.String())
				}
			}
		})
	}
}

// TestAPIClient is a GrafanaDetectorAPIClient implementation for testing.
type TestAPIClient struct {
	DashboardJSONFilePath    string
	DashboardMetaFilePath    string
	FrontendSettingsFilePath string
	DatasourcesFilePath      string
	PluginsFilePath          string
}

func NewTestAPIClient(dashboardJSONFilePath string) *TestAPIClient {
	return &TestAPIClient{
		DashboardJSONFilePath:    dashboardJSONFilePath,
		DashboardMetaFilePath:    filepath.Join("testdata", "dashboard-meta.json"),
		FrontendSettingsFilePath: filepath.Join("testdata", "frontend-settings.json"),
		DatasourcesFilePath:      filepath.Join("testdata", "datasources.json"),
		PluginsFilePath:          filepath.Join("testdata", "plugins.json"),
	}
}

// unmarshalFromFile unmarshals JSON from a file into out, which must be a pointer to a value.
func unmarshalFromFile(fn string, out any) error {
	f, err := os.Open(fn)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return json.NewDecoder(f).Decode(out)
}

// BaseURL always returns an empty string.
func (c *TestAPIClient) BaseURL() string {
	return ""
}

// GetPlugins returns plugins the content of c.PluginsFilePath.
func (c *TestAPIClient) GetPlugins(_ context.Context) (plugins []grafana.Plugin, err error) {
	err = unmarshalFromFile(c.PluginsFilePath, &plugins)
	return
}

// GetDatasourcePluginIDs returns the content of c.DatasourcesFilePath.
func (c *TestAPIClient) GetDatasourcePluginIDs(_ context.Context) (datasources []grafana.Datasource, err error) {
	err = unmarshalFromFile(c.DatasourcesFilePath, &datasources)
	return
}

// GetDashboards returns a dummy response with only one dashboard.
func (c *TestAPIClient) GetDashboards(_ context.Context, page int) ([]grafana.ListedDashboard, error) {
	if page > 1 {
		// Only 1 page, return empty list for all other pages.
		return []grafana.ListedDashboard{}, nil
	}
	return []grafana.ListedDashboard{
		{
			UID:   "test-case-dashboard",
			URL:   "/d/test-case-dashboard/test-case-dashboard",
			Title: "test case dashboard",
		},
	}, nil
}

// GetDashboard returns a new DashboardDefinition that can be used for testing purposes.
// The dashboard definition is taken from the file specified in c.DashboardJSONFilePath.
// The dashboard meta is taken from the file specified in c.DashboardMetaFilePath.
func (c *TestAPIClient) GetDashboard(_ context.Context, _ string) (*grafana.DashboardDefinition, error) {
	if c.DashboardJSONFilePath == "" {
		return nil, fmt.Errorf("TestAPIClient DashboardJSONFilePath cannot be empty")
	}
	var out grafana.DashboardDefinition
	if err := unmarshalFromFile(c.DashboardMetaFilePath, &out); err != nil {
		return nil, fmt.Errorf("unmarshal meta: %w", err)
	}
	if err := unmarshalFromFile(c.DashboardJSONFilePath, &out.Dashboard); err != nil {
		return nil, fmt.Errorf("unmarshal dashboard: %w", err)
	}
	grafana.ConvertPanels(out.Dashboard.Panels)
	return &out, nil
}

// GetFrontendSettings returns the content of c.FrontendSettingsFilePath.
func (c *TestAPIClient) GetFrontendSettings(_ context.Context) (frontendSettings *grafana.FrontendSettings, err error) {
	err = unmarshalFromFile(c.FrontendSettingsFilePath, &frontendSettings)
	return
}

// GetServiceAccountPermissions is not implemented for testing purposes and always returns an empty map and a nil error.
func (c *TestAPIClient) GetServiceAccountPermissions(_ context.Context) (map[string][]string, error) {
	return nil, nil
}

// static check
var _ GrafanaDetectorAPIClient = &TestAPIClient{}
