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

type DashboardPanel struct {
	Type       string
	Title      string
	Datasource interface{}

	Panels []*DashboardPanel // present for collapsed rows
}

type DashboardDefinition struct {
	Dashboard Dashboard `json:"dashboard"`
	Meta      Meta      `json:"meta"`
}

type Dashboard struct {
	Panels        []*DashboardPanel `json:"panels"`
	SchemaVersion int               `json:"schemaVersion"`
}
type Meta struct {
	Slug        string `json:"slug"`
	UpdatedBy   string `json:"updatedBy"`
	CreatedBy   string `json:"createdBy"`
	Created     string `json:"created"`
	Updated     string `json:"updated"`
	FolderUID   string `json:"folderUid"`
	FolderTitle string `json:"folderTitle"`
	FolderURL   string `json:"folderUrl"`
}

type Org struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
}
