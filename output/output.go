package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/grafana/detect-angular-dashboards/logger"
)

type DetectionType string

const (
	DetectionTypePanel      DetectionType = "panel"
	DetectionTypeDatasource DetectionType = "datasource"
)

type Detection struct {
	PluginID      string
	DetectionType DetectionType
	Title         string
}

func (d Detection) String() string {
	if d.DetectionType == DetectionTypePanel {
		return fmt.Sprintf("Found panel with angular data source %q (%q)", d.Title, d.PluginID)
	}
	return fmt.Sprintf("Found angular panel %q (%q)", d.Title, d.PluginID)
}

type Dashboard struct {
	Detections []Detection
	URL        string
	Title      string
}

type Outputter interface {
	Output([]Dashboard) error
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

type JSONOutputter struct {
	writer io.Writer
}

func NewJSONOutputter(w io.Writer) JSONOutputter {
	return JSONOutputter{writer: w}
}

func (o JSONOutputter) Output(v []Dashboard) error {
	var j int
	for i, dashboard := range v {
		// Remove dashboards without detections
		if len(dashboard.Detections) == 0 {
			continue
		}
		v[j] = v[i]
		j++
	}
	v = v[:j]
	enc := json.NewEncoder(o.writer)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
