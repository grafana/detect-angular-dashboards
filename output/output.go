package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/grafana/detect-angular-dashboards/logger"
)

type DetectionType string

const (
	DetectionTypePanel       DetectionType = "panel"
	DetectionTypeDatasource  DetectionType = "datasource"
	DetectionTypeLegacyPanel DetectionType = "legacyPanel"
)

type Detection struct {
	// PluginID is the plugin ID that triggered the detection.
	PluginID string

	// DetectionType identifies the type of the detection.
	DetectionType DetectionType

	// Title is the title of the panel that triggered the detection.
	// It is used so the user can identify the panel on the dashboard.
	Title string
}

func (d Detection) String() string {
	switch d.DetectionType {
	case DetectionTypePanel:
		return fmt.Sprintf("Found angular panel %q (%q)", d.Title, d.PluginID)
	case DetectionTypeDatasource:
		return fmt.Sprintf("Found panel with angular data source %q (%q)", d.Title, d.PluginID)
	case DetectionTypeLegacyPanel:
		return fmt.Sprintf(`Found legacy plugin %q in panel %q. `+
			`It can be migrated to a React-based panel by Grafana when opening the dashboard.`,
			d.PluginID,
			d.Title,
		)
	}
	return ""
}

type Dashboard struct {
	Detections []Detection
	URL        string
	Title      string
	Folder     string
	UpdatedBy  string
	CreatedBy  string
	Created    string
	Updated    string
}

type Outputter interface {
	Output([]Dashboard) error
	BulkOutput(map[int][]Dashboard) error
}

type LoggerReadableOutput struct {
	log *logger.LeveledLogger
}

func NewLoggerReadableOutput(log *logger.LeveledLogger) LoggerReadableOutput {
	return LoggerReadableOutput{log: log}
}

func (o LoggerReadableOutput) Output(v []Dashboard) error {
	for _, dashboard := range v {
		if len(dashboard.Detections) == 0 {
			o.log.Verbose().Log("Checking dashboard %q %q", dashboard.Title, dashboard.URL)
			continue
		}
		o.log.Log("Found dashboard with Angular plugins %q %q:", dashboard.Title, dashboard.URL)
		for _, detection := range dashboard.Detections {
			o.log.Log(detection.String())
		}
	}
	return nil
}

func (o LoggerReadableOutput) BulkOutput(v map[int][]Dashboard) error {
	for org, dashboards := range v {
		if len(dashboards) > 0 {
			o.log.Log("Found dashboards with Angular plugins in org %d", org)
			err := o.Output(dashboards)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type JSONOutputter struct {
	writer io.Writer
}

func NewJSONOutputter(w io.Writer) JSONOutputter {
	return JSONOutputter{writer: w}
}

func (o JSONOutputter) Output(v []Dashboard) error {
	enc := json.NewEncoder(o.writer)
	enc.SetIndent("", "  ")
	return enc.Encode(o.removeDashboardsWithoutDetections(v))
}

func (o JSONOutputter) removeDashboardsWithoutDetections(v []Dashboard) []Dashboard {
	var j int
	for i, dashboard := range v {
		// Remove dashboards without detections
		if len(dashboard.Detections) == 0 {
			continue
		}
		v[j] = v[i]
		j++
	}
	return v[:j]
}

func (o JSONOutputter) BulkOutput(v map[int][]Dashboard) error {
	for orgID, dashboards := range v {
		if len(dashboards) == 0 {
			delete(v, orgID)
		} else {
			v[orgID] = o.removeDashboardsWithoutDetections(dashboards)
		}
	}
	enc := json.NewEncoder(o.writer)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
