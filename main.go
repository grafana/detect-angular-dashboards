package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/grafana/detect-angular-dashboards/api/gcom"
	"github.com/grafana/detect-angular-dashboards/api/grafana"
	"github.com/grafana/detect-angular-dashboards/build"
	"github.com/grafana/detect-angular-dashboards/logger"
	"github.com/grafana/detect-angular-dashboards/output"
)

const (
	envGrafanaToken = "GRAFANA_TOKEN"

	pluginIDGraphOld = "graph"
	pluginIDTable    = "table"
	pluginIDTableOld = "table-old"
)

func _main() error {
	versionFlag := flag.Bool("version", false, "print version number")
	verboseFlag := flag.Bool("v", false, "verbose output")
	jsonOutputFlag := flag.Bool("j", false, "json output")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("%s %s (%s)\n", os.Args[0], build.LinkerVersion, build.LinkerCommitSHA)
		return nil
	}

	log := logger.NewLeveledLogger(*verboseFlag)
	if *jsonOutputFlag {
		// Redirect everything to stderr to avoid mixing with json output
		log.Logger.SetOutput(os.Stderr)
		log.WarnLogger.SetOutput(os.Stderr)
	}

	token := os.Getenv(envGrafanaToken)
	if token == "" {
		return fmt.Errorf("missing env var %q", envGrafanaToken)
	}
	grafanaURL := grafana.DefaultBaseURL
	if flag.NArg() >= 1 {
		grafanaURL = flag.Arg(0)
	}
	log.Log("Detecting Angular dashboards for %q", grafanaURL)

	ctx := context.Background()
	grCl := grafana.NewAPIClient(grafanaURL, token)
	gcomCl := gcom.NewAPIClient()

	// Determine if plugins are angular.
	// This can be done from frontendsettings (faster and works with private plugins, but only works with >= 10.1.0)
	// or from GCOM (slower, but always available, but public plugins only)
	angularDetected := map[string]bool{}
	frontendSettings, err := grCl.GetFrontendSettings(ctx)
	if err != nil {
		return fmt.Errorf("get frontend settings: %w", err)
	}

	// Determine if we should use GCOM or frontendsettings
	var useGCOM bool
	// Get any key and see if AngularDetected is present or not.
	// From Grafana 10.1.0, it will always be present.
	// Before Grafana 10.1.0, it's always nil as it's not present in the body.
	for _, p := range frontendSettings.Panels {
		useGCOM = p.AngularDetected == nil
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
		permissions, err := grCl.GetServiceAccountPermissions(ctx)
		if err != nil {
			// Do not hard fail if we can't get service account permissions
			// as we may be running against an old Grafana version without service accounts
			log.Verbose().Log("(WARNING: could not get service account permissions: %v)", err)
			log.Verbose().Log("Please make sure that you have created an ADMIN token or the output will be wrong")
		} else {
			_, hasDsCreate := permissions["datasources:create"]
			_, hasPluginsInstall := permissions["plugins:install"]
			if !hasDsCreate && !hasPluginsInstall {
				return fmt.Errorf(
					`the service account does not have "datasources:create" or "plugins:install" permission, ` +
						"please provide a token for a service account with admin privileges",
				)
			}
		}

		// Get the plugins
		plugins, err := grCl.GetPlugins(ctx)
		if err != nil {
			return fmt.Errorf("get plugins: %w", err)
		}
		for _, p := range plugins {
			if p.Info.Version == "" {
				continue
			}
			angularDetected[p.ID], err = gcomCl.GetAngularDetected(ctx, p.ID, p.Info.Version)
			if err != nil {
				return fmt.Errorf("get angular detected: %w", err)
			}
		}
	} else {
		log.Verbose().Log("Using frontendsettings to find Angular plugins")
		for pluginID, meta := range frontendSettings.Panels {
			angularDetected[pluginID] = *meta.AngularDetected
		}
		for _, meta := range frontendSettings.Datasources {
			angularDetected[meta.Type] = *meta.AngularDetected
		}
	}

	// Debug
	for p, isAngular := range angularDetected {
		log.Verbose().Log("Plugin %q angular %t", p, isAngular)
	}

	// Map ds name -> ds plugin id, to resolve legacy dashboards that have ds name
	apiDs, err := grCl.GetDatasourcePluginIDs(ctx)
	if err != nil {
		return fmt.Errorf("get datasource plugin ids: %w", err)
	}
	datasourcePluginIDs := make(map[string]string, len(apiDs))
	for _, ds := range apiDs {
		datasourcePluginIDs[ds.Name] = ds.Type
		log.Verbose().Log("Datasource %q plugin ID %q", ds.Name, ds.Type)
	}

	dashboards, err := grCl.GetDashboards(ctx, 1)
	if err != nil {
		return fmt.Errorf("get dashboards: %w", err)
	}

	var out output.Outputter
	if *jsonOutputFlag {
		out = output.NewJSONOutputter(os.Stdout)
	} else {
		out = output.NewLoggerReadableOutput(log)
	}
	var finalOutput []output.Dashboard
	for _, d := range dashboards {
		// Determine absolute dashboard URL for output
		dashboardAbsURL, err := url.JoinPath(strings.TrimSuffix(grafanaURL, "/api"), d.URL)
		if err != nil {
			// Silently ignore errors
			dashboardAbsURL = ""
		}
		dashboardOutput := output.Dashboard{
			Detections: []output.Detection{},
			URL:        dashboardAbsURL,
			Title:      d.Title,
		}
		dashboard, err := grCl.GetDashboard(ctx, d.UID)
		if err != nil {
			return fmt.Errorf("get dashboard %q: %w", d.UID, err)
		}
		for _, p := range dashboard.Panels {
			// Check panel
			// - "graph" has been replaced with timeseries
			// - "table-old" is the old table panel (after it has been migrated)
			// - "table" with a schema version < 24 is Angular table panel, which will be replaced by `table-old`:
			//		https://github.com/grafana/grafana/blob/7869ca1932c3a2a8f233acf35a3fe676187847bc/public/app/features/dashboard/state/DashboardMigrator.ts#L595-L610
			if p.Type == pluginIDGraphOld || p.Type == pluginIDTableOld || (p.Type == pluginIDTable && dashboard.SchemaVersion < 24) {
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
				return fmt.Errorf("unknown unmarshaled datasource type %T", p.Datasource)
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
	// Print output
	if err := out.Output(finalOutput); err != nil {
		return fmt.Errorf("output: %w", err)
	}
	return nil
}

func main() {
	if err := _main(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", err)
	}
}
