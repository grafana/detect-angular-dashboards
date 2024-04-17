package detector

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/grafana/detect-angular-dashboards/api/gcom"
	"github.com/grafana/detect-angular-dashboards/api/grafana"
	"github.com/grafana/detect-angular-dashboards/logger"
	"github.com/grafana/detect-angular-dashboards/output"
)

const (
	pluginIDGraphOld = "graph"
	pluginIDTable    = "table"
	pluginIDTableOld = "table-old"
)

// Run runs the angular detector tool against the specified Grafana instance.
func Run(ctx context.Context, log *logger.LeveledLogger, grafanaClient grafana.APIClient, orgID int) ([]output.Dashboard, error) {
	var (
		finalOutput []output.Dashboard
		// Determine if we should use GCOM or frontendsettings
		useGCOM bool
	)

	gcomCl := gcom.NewAPIClient()

	// Determine if plugins are angular.
	// This can be done from frontendsettings (faster and works with private plugins, but only works with >= 10.1.0)
	// or from GCOM (slower, but always available, but public plugins only)
	angularDetected := map[string]bool{}
	frontendSettings, err := grafanaClient.GetFrontendSettings(ctx)
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
		log.Verbose().Log("Using GCOM to find Angular plugins")
		log.Log("(WARNING, dependencies on private plugins won't be flagged)")

		// Double check that the token has the correct permissions, which is "datasources:create".
		// If we don't have such permissions, the plugins endpoint will still return a valid response,
		// but it will contain only core plugins:
		// https://github.com/grafana/grafana/blob/0315b911ef45b4ce9d3d5c182d8b112c6b9b41da/pkg/api/plugins.go#L56
		permissions, err := grafanaClient.GetServiceAccountPermissions(ctx)
		if err != nil {
			// Do not hard fail if we can't get service account permissions
			// as we may be running against an old Grafana version without service accounts
			log.Verbose().Log("(WARNING: could not get service account permissions: %v)", err)
			log.Verbose().Log("Please make sure that you have created an ADMIN token or the output will be wrong")
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
		plugins, err := grafanaClient.GetPlugins(ctx)
		if err != nil {
			return []output.Dashboard{}, fmt.Errorf("get plugins: %w", err)
		}
		for _, p := range plugins {
			if p.Info.Version == "" {
				continue
			}
			angularDetected[p.ID], err = gcomCl.GetAngularDetected(ctx, p.ID, p.Info.Version)
			if err != nil {
				return []output.Dashboard{}, fmt.Errorf("get angular detected: %w", err)
			}
		}
	} else {
		log.Verbose().Log("Using frontendsettings to find Angular plugins")
		for pluginID, panel := range frontendSettings.Panels {
			v, err := panel.IsAngular()
			if err != nil {
				return []output.Dashboard{}, fmt.Errorf("%q is angular: %w", pluginID, err)
			}
			angularDetected[pluginID] = v
		}
		for _, ds := range frontendSettings.Datasources {
			v, err := ds.IsAngular()
			if err != nil {
				return []output.Dashboard{}, fmt.Errorf("%q is angular: %w", ds.Type, err)
			}
			angularDetected[ds.Type] = v
		}
	}

	// Debug
	for p, isAngular := range angularDetected {
		log.Verbose().Log("Plugin %q angular %t", p, isAngular)
	}

	// Map ds name -> ds plugin id, to resolve legacy dashboards that have ds name
	apiDs, err := grafanaClient.GetDatasourcePluginIDs(ctx)
	if err != nil {
		return []output.Dashboard{}, fmt.Errorf("get datasource plugin ids: %w", err)
	}
	datasourcePluginIDs := make(map[string]string, len(apiDs))
	for _, ds := range apiDs {
		datasourcePluginIDs[ds.Name] = ds.Type
	}

	dashboards, err := grafanaClient.GetDashboards(ctx, 1)
	if err != nil {
		return []output.Dashboard{}, fmt.Errorf("get dashboards: %w", err)
	}

	orgIDURLsuffix := "?" + url.Values{
		"orgID": []string{strconv.Itoa(orgID)},
	}.Encode()

	for _, d := range dashboards {
		// Determine absolute dashboard URL for output

		dashboardAbsURL, err := url.JoinPath(strings.TrimSuffix(grafanaClient.BaseURL, "/api"), d.URL+orgIDURLsuffix)
		if err != nil {
			// Silently ignore errors
			dashboardAbsURL = ""
		}
		dashboardDefinition, err := grafanaClient.GetDashboard(ctx, d.UID)
		if err != nil {
			return []output.Dashboard{}, fmt.Errorf("get dashboard %q: %w", d.UID, err)
		}
		dashboardOutput := output.Dashboard{
			Detections: []output.Detection{},
			URL:        dashboardAbsURL,
			Title:      d.Title,
			Folder:     dashboardDefinition.Meta.FolderTitle,
			CreatedBy:  dashboardDefinition.Meta.CreatedBy,
			UpdatedBy:  dashboardDefinition.Meta.UpdatedBy,
			Created:    dashboardDefinition.Meta.Created,
			Updated:    dashboardDefinition.Meta.Updated,
		}
		for _, p := range dashboardDefinition.Dashboard.Panels {
			// Check panel
			// - "graph" has been replaced with timeseries
			// - "table-old" is the old table panel (after it has been migrated)
			// - "table" with a schema version < 24 is Angular table panel, which will be replaced by `table-old`:
			//		https://github.com/grafana/grafana/blob/7869ca1932c3a2a8f233acf35a3fe676187847bc/public/app/features/dashboard/state/DashboardMigrator.ts#L595-L610
			if p.Type == pluginIDGraphOld || p.Type == pluginIDTableOld || (p.Type == pluginIDTable && dashboardDefinition.Dashboard.SchemaVersion < 24) {
				// Different warning on legacy panel that can be migrated to React automatically
				dashboardOutput.Detections = append(dashboardOutput.Detections, output.Detection{
					DetectionType: output.DetectionTypeLegacyPanel,
					PluginID:      p.Type,
					Title:         p.Title,
				})
			} else if angularDetected[p.Type] {
				// Angular plugin
				dashboardOutput.Detections = append(dashboardOutput.Detections, output.Detection{
					DetectionType: output.DetectionTypePanel,
					PluginID:      p.Type,
					Title:         p.Title,
				})
			}

			// Check datasource
			var dsPlugin string
			// The datasource field can either be a string (old) or object (new)
			if p.Datasource == nil || p.Datasource == "" {
				continue
			}
			if dsName, ok := p.Datasource.(string); ok {
				dsPlugin = datasourcePluginIDs[dsName]
			} else if ds, ok := p.Datasource.(grafana.PanelDatasource); ok {
				dsPlugin = ds.Type
			} else {
				return []output.Dashboard{}, fmt.Errorf("unknown unmarshaled datasource type %T", p.Datasource)
			}
			if angularDetected[dsPlugin] {
				dashboardOutput.Detections = append(dashboardOutput.Detections, output.Detection{
					DetectionType: output.DetectionTypeDatasource,
					PluginID:      dsPlugin,
					Title:         p.Title,
				})
			}
		}
		finalOutput = append(finalOutput, dashboardOutput)
	}
	return finalOutput, nil
}
