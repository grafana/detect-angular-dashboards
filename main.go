package main

import (
	"context"
	"fmt"
	"os"

	"github.com/grafana/detect-angular-dashboards/api/grafana"
)

const envGrafanaToken = "GRAFANA_TOKEN"

func _main() error {
	token := os.Getenv(envGrafanaToken)
	if token == "" {
		return fmt.Errorf("missing env var %q", envGrafanaToken)
	}
	grafanaURL := grafana.DefaultBaseURL
	if len(os.Args) >= 2 {
		grafanaURL = os.Args[1]
	}
	fmt.Printf("Detecting Angular dashboards for %q", grafanaURL)

	ctx := context.Background()
	grCl := grafana.NewAPIClient(grafanaURL, token)
	/* gcomCl := gcom.NewAPIClient()

	plugins, err := grCl.GetPlugins(ctx)
	if err != nil {
		return fmt.Errorf("get plugins: %w", err)
	}

	// Determine if plugins are angular
	angularDetected := make(map[string]bool, len(plugins))
	for _, p := range plugins {
		if p.Info.Version == "" {
			continue
		}
		angularDetected[p.ID], err = gcomCl.GetAngularDetected(ctx, p.ID, p.Info.Version)
		if err != nil {
			return fmt.Errorf("get angular detected: %w", err)
		}
		fmt.Println("Plugin", p.ID, "version", p.Info.Version, "angular", angularDetected[p.ID])
	}	 */

	// Determine if plugins are angular
	angularDetected := map[string]bool{}
	frontendSettings, err := grCl.GetFrontendSettings(ctx)
	if err != nil {
		return fmt.Errorf("get frontend settings: %w", err)
	}
	for pluginID, meta := range frontendSettings.Panels {
		angularDetected[pluginID] = meta.AngularDetected
	}
	for _, meta := range frontendSettings.Datasources {
		angularDetected[meta.Type] = meta.AngularDetected
	}

	// Map ds name -> ds plugin id, to resolve legacy dashboards that have ds name
	apiDs, err := grCl.GetDatasourcePluginIDs(ctx)
	if err != nil {
		return fmt.Errorf("get datasource plugin ids: %w", err)
	}
	datasourcePluginIDs := make(map[string]string, len(apiDs))
	for _, ds := range apiDs {
		datasourcePluginIDs[ds.Name] = ds.Type
		fmt.Println("Datasource", ds.Name, "plugin id", ds.Type)
	}

	dashboards, err := grCl.GetDashboards(ctx, 1)
	if err != nil {
		return fmt.Errorf("get dashboards: %w", err)
	}
	for _, d := range dashboards {
		fmt.Println("Checking dashboard", d.UID, d.Title)
		dashboard, err := grCl.GetDashboard(ctx, d.UID)
		if err != nil {
			return fmt.Errorf("get dashboard %q: %w", d.UID, err)
		}
		for _, p := range dashboard.Panels {
			// Check panel
			if angularDetected[p.Type] {
				fmt.Println("\tFound angular panel", p.Type)
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
				fmt.Println("\tFound angular datasource", dsPlugin)
			}
		}
	}

	return nil
}

func main() {
	if err := _main(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", err)
	}
}
