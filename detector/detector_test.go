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
		cl := TestAPIClient{
			DashboardJSONFilePath: filepath.Join("testdata", "dashboards", "graph-old.json"),
		}
		d := NewDetector(logger.NewLeveledLogger(false), &cl, gcom.NewAPIClient())
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

	t.Run("legacy panel", func(t *testing.T) {
		cl := TestAPIClient{
			DashboardJSONFilePath: filepath.Join("testdata", "dashboards", "graph-old.json"),
		}
		d := NewDetector(logger.NewLeveledLogger(false), &cl, gcom.NewAPIClient())
		out, err := d.Run(context.Background())
		require.NoError(t, err)
		require.Len(t, out, 1)
		require.Len(t, out[0].Detections, 1)
		require.Equal(t, "graph", out[0].Detections[0].PluginID)
		require.Equal(t, output.DetectionTypeLegacyPanel, out[0].Detections[0].DetectionType)
		require.Equal(t, "Flot graph", out[0].Detections[0].Title)
		require.Equal(
			t,
			`Found legacy plugin "graph" in panel "Flot graph". It can be migrated to a React-based panel by Grafana when opening the dashboard.`,
			out[0].Detections[0].String(),
		)
	})

	t.Run("angular panel", func(t *testing.T) {
		cl := TestAPIClient{
			DashboardJSONFilePath: filepath.Join("testdata", "dashboards", "worldmap.json"),
		}
		d := NewDetector(logger.NewLeveledLogger(false), &cl, gcom.NewAPIClient())
		out, err := d.Run(context.Background())
		require.NoError(t, err)
		require.Len(t, out, 1)
		require.Len(t, out[0].Detections, 1)
		require.Equal(t, "grafana-worldmap-panel", out[0].Detections[0].PluginID)
		require.Equal(t, output.DetectionTypePanel, out[0].Detections[0].DetectionType)
		require.Equal(t, "Panel Title", out[0].Detections[0].Title)
		require.Equal(t, `Found angular panel "Panel Title" ("grafana-worldmap-panel")`, out[0].Detections[0].String())
	})

	t.Run("datasource", func(t *testing.T) {
		cl := TestAPIClient{
			DashboardJSONFilePath: filepath.Join("testdata", "dashboards", "datasource.json"),
		}
		d := NewDetector(logger.NewLeveledLogger(false), &cl, gcom.NewAPIClient())
		out, err := d.Run(context.Background())
		require.NoError(t, err)
		require.Len(t, out, 1)
		require.Len(t, out[0].Detections, 1)
		require.Equal(t, "akumuli-datasource", out[0].Detections[0].PluginID)
		require.Equal(t, output.DetectionTypeDatasource, out[0].Detections[0].DetectionType)
		require.Equal(t, "akumuli", out[0].Detections[0].Title)
		require.Equal(t, `Found panel with angular data source "akumuli" ("akumuli-datasource")`, out[0].Detections[0].String())
	})

	t.Run("not angular", func(t *testing.T) {
		cl := TestAPIClient{
			DashboardJSONFilePath: filepath.Join("testdata", "dashboards", "not-angular.json"),
		}
		d := NewDetector(logger.NewLeveledLogger(false), &cl, gcom.NewAPIClient())
		out, err := d.Run(context.Background())
		require.NoError(t, err)
		require.Len(t, out, 1)
		require.Empty(t, out[0].Detections)
	})

	t.Run("multiple", func(t *testing.T) {
		cl := TestAPIClient{
			DashboardJSONFilePath: filepath.Join("testdata", "dashboards", "multiple.json"),
		}
		d := NewDetector(logger.NewLeveledLogger(false), &cl, gcom.NewAPIClient())
		out, err := d.Run(context.Background())
		require.NoError(t, err)
		require.Len(t, out, 1)
		require.Len(t, out[0].Detections, 4)

		exp := []struct {
			pluginID      string
			detectionType output.DetectionType
			title         string
		}{
			{"akumuli-datasource", output.DetectionTypeDatasource, "akumuli"},
			{"grafana-worldmap-panel", output.DetectionTypePanel, "worldmap + akumuli"},
			{"akumuli-datasource", output.DetectionTypeDatasource, "worldmap + akumuli"},
			{"graph", output.DetectionTypeLegacyPanel, "graph-old"},
		}
		for i, e := range exp {
			require.Equalf(t, e.pluginID, out[0].Detections[i].PluginID, "plugin id %d", i)
			require.Equalf(t, e.detectionType, out[0].Detections[i].DetectionType, "detection type %d", i)
			require.Equalf(t, e.title, out[0].Detections[i].Title, "title %d", i)
		}
	})

	t.Run("mixed", func(t *testing.T) {
		// mix of angular and react panels

		cl := TestAPIClient{
			DashboardJSONFilePath: filepath.Join("testdata", "dashboards", "mixed.json"),
		}
		d := NewDetector(logger.NewLeveledLogger(false), &cl, gcom.NewAPIClient())
		out, err := d.Run(context.Background())
		require.NoError(t, err)
		require.Len(t, out, 1)
		require.Len(t, out[0].Detections, 1)
		require.Equal(t, "angular", out[0].Detections[0].Title)
	})

	t.Run("rows", func(t *testing.T) {
		t.Run("expanded", func(t *testing.T) {
			cl := TestAPIClient{
				DashboardJSONFilePath: filepath.Join("testdata", "dashboards", "rows-expanded.json"),
			}
			d := NewDetector(logger.NewLeveledLogger(false), &cl, gcom.NewAPIClient())
			out, err := d.Run(context.Background())
			require.NoError(t, err)
			require.Len(t, out, 1)
			require.Len(t, out[0].Detections, 1)
			require.Equal(t, "expanded", out[0].Detections[0].Title)
		})

		t.Run("collapsed", func(t *testing.T) {
			cl := TestAPIClient{
				DashboardJSONFilePath: filepath.Join("testdata", "dashboards", "rows-collapsed.json"),
			}
			d := NewDetector(logger.NewLeveledLogger(false), &cl, gcom.NewAPIClient())
			out, err := d.Run(context.Background())
			require.NoError(t, err)
			require.Len(t, out, 1)
			require.Len(t, out[0].Detections, 1)
			require.Equal(t, "collapsed", out[0].Detections[0].Title)
		})
	})
}

type TestAPIClient struct {
	DashboardJSONFilePath    string
	DashboardMetaFilePath    string
	FrontendSettingsFilePath string
	DatasourcesFilePath      string
	PluginsFilePath          string
}

func unmarshalFromFile(fn string, out any) error {
	f, err := os.Open(fn)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(out)
}

func (c *TestAPIClient) BaseURL() string {
	return ""
}

func (c *TestAPIClient) GetPlugins(ctx context.Context) (plugins []grafana.Plugin, err error) {
	fn := c.PluginsFilePath
	if fn == "" {
		fn = filepath.Join("testdata", "plugins.json")
	}
	err = unmarshalFromFile(fn, &plugins)
	return
}

func (c *TestAPIClient) GetDatasourcePluginIDs(ctx context.Context) (datasources []grafana.Datasource, err error) {
	fn := c.DatasourcesFilePath
	if fn == "" {
		fn = filepath.Join("testdata", "datasources.json")
	}
	err = unmarshalFromFile(fn, &datasources)
	return
}

func (c *TestAPIClient) GetDashboards(ctx context.Context, page int) ([]grafana.ListedDashboard, error) {
	return []grafana.ListedDashboard{
		{
			UID:   "test-case-dashboard",
			URL:   "/d/test-case-dashboard/test-case-dashboard",
			Title: "test case dashboard",
		},
	}, nil
}

func (c *TestAPIClient) GetDashboard(ctx context.Context, uid string) (*grafana.DashboardDefinition, error) {
	if c.DashboardJSONFilePath == "" {
		return nil, fmt.Errorf("TestAPIClient DashboardJSONFilePath cannot be empty")
	}
	metaFn := c.DashboardMetaFilePath
	if metaFn == "" {
		metaFn = filepath.Join("testdata", "dashboard-meta.json")
	}
	var out grafana.DashboardDefinition
	if err := unmarshalFromFile(metaFn, &out); err != nil {
		return nil, fmt.Errorf("unmarshal meta: %w", err)
	}
	if err := unmarshalFromFile(c.DashboardJSONFilePath, &out.Dashboard); err != nil {
		return nil, fmt.Errorf("unmarshal dashboard: %w", err)
	}
	grafana.ConvertPanels(out.Dashboard.Panels)
	return &out, nil
}

func (c *TestAPIClient) GetFrontendSettings(ctx context.Context) (frontendSettings *grafana.FrontendSettings, err error) {
	fn := c.FrontendSettingsFilePath
	if fn == "" {
		fn = filepath.Join("testdata", "frontend-settings.json")
	}
	err = unmarshalFromFile(fn, &frontendSettings)
	return
}

func (c *TestAPIClient) GetServiceAccountPermissions(ctx context.Context) (map[string][]string, error) {
	return nil, nil
}

// static check
var _ GrafanaDetectorAPIClient = &TestAPIClient{}
