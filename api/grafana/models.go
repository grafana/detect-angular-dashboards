package grafana

type PluginInfo struct {
	Version string `json:"version"`
}

type Plugin struct {
	ID   string
	Info PluginInfo
}

type Datasource struct {
	Name string
	Type string
}

type ListedDashboard struct {
	UID   string
	URL   string
	Title string
}

type PanelDatasource struct {
	Type string
}

type Panel struct {
	Type       string
	Title      string
	Datasource interface{}
}

type Dashboard struct {
	Panels        []*Panel `json:"panels"`
	SchemaVersion int      `json:"schemaVersion"`
}
