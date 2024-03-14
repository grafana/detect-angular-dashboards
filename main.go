package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/grafana/detect-angular-dashboards/api/grafana"
	"github.com/grafana/detect-angular-dashboards/build"
	"github.com/grafana/detect-angular-dashboards/detector"
	"github.com/grafana/detect-angular-dashboards/logger"
	"github.com/grafana/detect-angular-dashboards/output"
)

const (
	envGrafanaToken = "GRAFANA_TOKEN"
)

func newLogger(verboseFlag, jsonOutputFlag bool) *logger.LeveledLogger {
	log := logger.NewLeveledLogger(verboseFlag)
	if jsonOutputFlag {
		// Redirect everything to stderr to avoid mixing with json output
		log.Logger.SetOutput(os.Stderr)
		log.WarnLogger.SetOutput(os.Stderr)
	}
	return log
}

func getToken() (string, error) {
	token := os.Getenv(envGrafanaToken)
	if token == "" {
		return "", fmt.Errorf("missing env var %q", envGrafanaToken)
	}
	return token, nil
}

func main() {
	versionFlag := flag.Bool("version", false, "print version number")
	verboseFlag := flag.Bool("v", false, "verbose output")
	jsonOutputFlag := flag.Bool("j", false, "json output")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("%s %s (%s)\n", os.Args[0], build.LinkerVersion, build.LinkerCommitSHA)
		os.Exit(0)
	}
	log := newLogger(*verboseFlag, *jsonOutputFlag)

	token, err := getToken()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	ctx := context.Background()
	grafanaURL := grafana.DefaultBaseURL
	if flag.NArg() >= 1 {
		grafanaURL = flag.Arg(0)
	}

	log.Log("Detecting Angular dashboards for %q", grafanaURL)
	finalOutput, err := detector.Run(ctx, log, grafanaURL, token)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(0)
	}

	var out output.Outputter
	if *jsonOutputFlag {
		out = output.NewJSONOutputter(os.Stdout)
	} else {
		out = output.NewLoggerReadableOutput(log)
	}

	// Print output
	if err := out.Output(finalOutput); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "output: %s\n", err)
	}
}
