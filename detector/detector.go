package detector

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/grafana/detect-angular-dashboards/api/gcom"
	"github.com/grafana/detect-angular-dashboards/api/grafana"
	"github.com/grafana/detect-angular-dashboards/logger"
	"github.com/grafana/detect-angular-dashboards/output"
)

const (
	pluginIDGraphOld      = "graph"
	pluginIDTable         = "table"
	pluginIDTableOld      = "table-old"
	pluginIDPiechart      = "grafana-piechart-panel"
	pluginIDWorldmap      = "grafana-worldmap-panel"
	pluginIDSinglestatOld = "grafana-singlestat-panel"
	pluginIDSinglestat    = "singlestat"
)

// GrafanaDetectorAPIClient is an interface that can be used to interact with the Grafana API for
// detecting Angular plugins.
type GrafanaDetectorAPIClient interface {
	BaseURL() string
	GetPlugins(ctx context.Context) ([]grafana.Plugin, error)
	GetFrontendSettings(ctx context.Context) (*grafana.FrontendSettings, error)
	GetServiceAccountPermissions(ctx context.Context) (map[string][]string, error)
	GetDatasourcePluginIDs(ctx context.Context) ([]grafana.Datasource, error)
	GetDashboards(ctx context.Context, page int) ([]grafana.ListedDashboard, error)
	GetDashboard(ctx context.Context, uid string) (*grafana.DashboardDefinition, error)
}

// Detector can detect Angular plugins in Grafana dashboards.
type Detector struct {
	log           *logger.LeveledLogger
	grafanaClient GrafanaDetectorAPIClient
	gcomClient    gcom.APIClient

	angularDetected     map[string]bool
	datasourcePluginIDs map[string]string
	maxConcurrency      int
}

// NewDetector returns a new Detector.
func NewDetector(log *logger.LeveledLogger, grafanaClient GrafanaDetectorAPIClient, gcomClient gcom.APIClient, maxConcurrency int) *Detector {
	return &Detector{
		log:             log,
		grafanaClient:   grafanaClient,
		gcomClient:      gcomClient,
		angularDetected: map[string]bool{},
		maxConcurrency:  maxConcurrency,
	}
}

// Run runs the angular detector tool against the specified Grafana instance.
func (d *Detector) Run(ctx context.Context) ([]output.Dashboard, error) {
	var (
		finalOutput []output.Dashboard
		// Determine if we should use GCOM or frontendsettings
		useGCOM bool
	)

	// Determine if plugins are angular.
	// This can be done from frontendsettings (faster and works with private plugins, but only works with >= 10.1.0)
	// or from GCOM (slower, but always available, but public plugins only)
	frontendSettings, err := d.grafanaClient.GetFrontendSettings(ctx)
	if err != nil {
		return []output.Dashboard{}, fmt.Errorf("get frontend settings: %w", err)
	}

	// Determine if we should use GCOM or frontendsettings
	// Get any key and see if Angular or AngularDetected is present or not.
	// With Grafana >= 10.3.0, Angular is present.
	// With Grafana >= 10.1.0 && < 10.3.0, AngularDetected is present.
	// With Grafana <= 10.1.0, it's always nil as it's not present in the body.
	// In the last case, we can only rely on the data in GCOM.
	for _, p := range frontendSettings.Panels {
		useGCOM = p.Angular == nil && p.AngularDetected == nil
		break
	}
	if useGCOM {
		// Fall back to GCOM (< 10.1.0)
		d.log.Verbose().Log("Using GCOM to find Angular plugins")
		d.log.Log("(WARNING, dependencies on private plugins won't be flagged)")

		// Double check that the token has the correct permissions, which is "datasources:create".
		// If we don't have such permissions, the plugins endpoint will still return a valid response,
		// but it will contain only core plugins:
		// https://github.com/grafana/grafana/blob/0315b911ef45b4ce9d3d5c182d8b112c6b9b41da/pkg/api/plugins.go#L56
		permissions, err := d.grafanaClient.GetServiceAccountPermissions(ctx)
		if err != nil {
			// Do not hard fail if we can't get service account permissions
			// as we may be running against an old Grafana version without service accounts
			d.log.Verbose().Log("(WARNING: could not get service account permissions: %v)", err)
			d.log.Verbose().Log("Please make sure that you have created an ADMIN token or the output will be wrong")
		} else {
			_, hasDsCreate := permissions["datasources:create"]
			_, hasPluginsInstall := permissions["plugins:install"]
			if !hasDsCreate && !hasPluginsInstall {
				return []output.Dashboard{}, fmt.Errorf(
					`the service account does not have "datasources:create" or "plugins:install" permission, ` +
						"please provide a token for a service account with admin privileges",
				)
			}
		}

		// Get the plugins
		plugins, err := d.grafanaClient.GetPlugins(ctx)
		if err != nil {
			return []output.Dashboard{}, fmt.Errorf("get plugins: %w", err)
		}
		for _, p := range plugins {
			if p.Info.Version == "" {
				continue
			}
			d.angularDetected[p.ID], err = d.gcomClient.GetAngularDetected(ctx, p.ID, p.Info.Version)
			if err != nil {
				return []output.Dashboard{}, fmt.Errorf("get angular detected: %w", err)
			}
		}
	} else {
		d.log.Verbose().Log("Using frontendsettings to find Angular plugins")
		for pluginID, panel := range frontendSettings.Panels {
			v, err := panel.IsAngular()
			if err != nil {
				return []output.Dashboard{}, fmt.Errorf("%q is angular: %w", pluginID, err)
			}
			d.angularDetected[pluginID] = v
		}
		for _, ds := range frontendSettings.Datasources {
			v, err := ds.IsAngular()
			if err != nil {
				return []output.Dashboard{}, fmt.Errorf("%q is angular: %w", ds.Type, err)
			}
			d.angularDetected[ds.Type] = v
		}
	}

	// Debug
	for p, isAngular := range d.angularDetected {
		d.log.Verbose().Log("Plugin %q angular %t", p, isAngular)
	}

	// Map ds name -> ds plugin id, to resolve legacy dashboards that have ds name
	apiDs, err := d.grafanaClient.GetDatasourcePluginIDs(ctx)
	if err != nil {
		return []output.Dashboard{}, fmt.Errorf("get datasource plugin ids: %w", err)
	}
	d.datasourcePluginIDs = make(map[string]string, len(apiDs))
	for _, ds := range apiDs {
		d.datasourcePluginIDs[ds.Name] = ds.Type
	}

	var page int
	for {
		page += 1
		dashboards, err := d.grafanaClient.GetDashboards(ctx, page)
		if err != nil {
			return []output.Dashboard{}, fmt.Errorf("get dashboards: %w", err)
		}
		if len(dashboards) == 0 {
			break
		}

		// Create a semaphore to limit concurrency
		semaphore := make(chan struct{}, d.maxConcurrency)
		var wg sync.WaitGroup
		var mu sync.Mutex
		var downloadErrors []error

		for _, dash := range dashboards {
			wg.Add(1)
			go func(dash grafana.ListedDashboard) {
				defer wg.Done()
				semaphore <- struct{}{}        // Acquire semaphore
				defer func() { <-semaphore }() // Release semaphore

				dashboardAbsURL, err := url.JoinPath(strings.TrimSuffix(d.grafanaClient.BaseURL(), "/api"), dash.URL)
				if err != nil {
					dashboardAbsURL = ""
				}
				dashboardDefinition, err := d.grafanaClient.GetDashboard(ctx, dash.UID)
				if err != nil {
					mu.Lock()
					downloadErrors = append(downloadErrors, fmt.Errorf("get dashboard %q: %w", dash.UID, err))
					mu.Unlock()
					return
				}
				dashboardOutput := output.Dashboard{
					Detections: []output.Detection{},
					URL:        dashboardAbsURL,
					Title:      dash.Title,
					Folder:     dashboardDefinition.Meta.FolderTitle,
					CreatedBy:  dashboardDefinition.Meta.CreatedBy,
					UpdatedBy:  dashboardDefinition.Meta.UpdatedBy,
					Created:    dashboardDefinition.Meta.Created,
					Updated:    dashboardDefinition.Meta.Updated,
				}
				dashboardOutput.Detections, err = d.checkPanels(dashboardDefinition, dashboardDefinition.Dashboard.Panels)
				if err != nil {
					mu.Lock()
					downloadErrors = append(downloadErrors, fmt.Errorf("check panels: %w", err))
					mu.Unlock()
					return
				}
				mu.Lock()
				finalOutput = append(finalOutput, dashboardOutput)
				mu.Unlock()
			}(dash)
		}

		wg.Wait()

		if len(downloadErrors) > 0 {
			return finalOutput, fmt.Errorf("errors occurred during dashboard download: %v", downloadErrors)
		}
	}

	return finalOutput, nil
}

// checkPanels calls checkPanel recursively on the given panels.
func (d *Detector) checkPanels(dashboardDefinition *grafana.DashboardDefinition, panels []*grafana.DashboardPanel) ([]output.Detection, error) {
	var out []output.Detection
	for _, p := range panels {
		r, err := d.checkPanel(dashboardDefinition, p)
		if err != nil {
			return nil, err
		}
		out = append(out, r...)

		// Recurse
		if len(p.Panels) == 0 {
			continue
		}
		rr, err := d.checkPanels(dashboardDefinition, p.Panels)
		if err != nil {
			return nil, err
		}
		out = append(out, rr...)
	}
	return out, nil
}

// isLegacyPanel returns true if the panel is a legacy panel that can be automatically migrated to a React equivalent
// by core Grafana upon opening the dashboard in the browser.
func (d *Detector) isLegacyPanel(pluginType string, dashboardSchemaVersion int) bool {
	if slices.Contains([]string{
		// "graph" has been replaced with timeseries
		pluginIDGraphOld,
		// "table-old" is the old table panel (after it has been migrated)
		pluginIDTableOld,
		// "grafana-piechart-panel" can be migrated by core:
		// https://github.com/grafana/grafana/blob/2638de6aeb6d780a2f51dd78f54e0ef7fcc25a7d/public/app/plugins/panel/piechart/migrations.ts#L11 ?
		pluginIDPiechart,
		// "grafana-worldmap-panel" can be migrated to "geomap" by core:
		// https://github.com/grafana/grafana/blob/2638de6aeb6d780a2f51dd78f54e0ef7fcc25a7d/public/app/features/dashboard/state/getPanelPluginToMigrateTo.ts#L67-L72
		pluginIDWorldmap,
		// "singlestat" and "grafana-singlestat-panel" can also be auto-migrated by core:
		// https://github.com/grafana/grafana/blob/2638de6aeb6d780a2f51dd78f54e0ef7fcc25a7d/public/app/plugins/panel/stat/StatMigrations.ts#L16-L17
		pluginIDSinglestatOld,
		pluginIDSinglestat,
	}, pluginType) {
		return true
	}
	// "table" with a schema version < 24 is Angular table panel, which will be replaced by `table-old`
	// https://github.com/grafana/grafana/blob/7869ca1932c3a2a8f233acf35a3fe676187847bc/public/app/features/dashboard/state/DashboardMigrator.ts#L595-L610
	if pluginType == pluginIDTable && dashboardSchemaVersion < 24 {
		return true
	}
	return false
}

// checkPanel checks the given panel for Angular plugins.
func (d *Detector) checkPanel(dashboardDefinition *grafana.DashboardDefinition, p *grafana.DashboardPanel) ([]output.Detection, error) {
	var out []output.Detection

	// Check panel
	if d.isLegacyPanel(p.Type, dashboardDefinition.Dashboard.SchemaVersion) {
		// Different warning on legacy panel that can be migrated to React automatically
		out = append(out, output.Detection{
			DetectionType: output.DetectionTypeLegacyPanel,
			PluginID:      p.Type,
			Title:         p.Title,
		})
	} else if d.angularDetected[p.Type] {
		// Angular plugin
		out = append(out, output.Detection{
			DetectionType: output.DetectionTypePanel,
			PluginID:      p.Type,
			Title:         p.Title,
		})
	}

	// Check datasource
	var dsPlugin string
	// The datasource field can either be a string (old) or object (new)
	if p.Datasource == nil || p.Datasource == "" {
		return out, nil
	}
	if dsName, ok := p.Datasource.(string); ok {
		dsPlugin = d.datasourcePluginIDs[dsName]
	} else if ds, ok := p.Datasource.(grafana.PanelDatasource); ok {
		dsPlugin = ds.Type
	} else {
		return nil, fmt.Errorf("unknown unmarshaled datasource type %T", p.Datasource)
	}
	if d.angularDetected[dsPlugin] {
		out = append(out, output.Detection{
			DetectionType: output.DetectionTypeDatasource,
			PluginID:      dsPlugin,
			Title:         p.Title,
		})
	}
	return out, nil
}
