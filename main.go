package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/grafana/detect-angular-dashboards/api"
	"github.com/grafana/detect-angular-dashboards/api/gcom"
	"github.com/grafana/detect-angular-dashboards/api/grafana"
	"github.com/grafana/detect-angular-dashboards/build"
	"github.com/grafana/detect-angular-dashboards/detector"
	"github.com/grafana/detect-angular-dashboards/logger"
	"github.com/grafana/detect-angular-dashboards/output"
)

const envGrafanaToken = "GRAFANA_TOKEN"

// newLogger initializes a new leveled logger.
func newLogger(verbose, jsonOutputFlag bool) *logger.LeveledLogger {
	log := logger.NewLeveledLogger(verbose)
	if jsonOutputFlag {
		// Redirect everything to stderr to avoid mixing with json output
		log.Logger.SetOutput(os.Stderr)
		log.WarnLogger.SetOutput(os.Stderr)
	}
	return log
}

// getToken retrieves the Grafana token from the environment variable.
func getToken() (string, error) {
	token := os.Getenv(envGrafanaToken)
	if token == "" {
		return "", fmt.Errorf("missing env var %q", envGrafanaToken)
	}
	return token, nil
}

// runDetection performs the detection of Angular dashboards.
func runDetection(ctx context.Context, log *logger.LeveledLogger, client grafana.APIClient, jsonOutputFlag bool) {
	log.Log("Detecting Angular dashboards")

	d := detector.NewDetector(log, client, gcom.NewAPIClient())
	finalOutput, err := d.Run(ctx)
	if err != nil {
		log.Errorf("%s\n", err)
		return
	}

	var out output.Outputter
	if jsonOutputFlag {
		out = output.NewJSONOutputter(os.Stdout)
	} else {
		out = output.NewLoggerReadableOutput(log)
	}

	// Print output
	if err := out.Output(finalOutput); err != nil {
		log.Errorf("output: %s\n", err)
	}
}

// parseFlags parses the command-line flags.
func parseFlags() (bool, bool, bool, bool, time.Duration) {
	versionFlag := flag.Bool("version", false, "print version number")
	verboseFlag := flag.Bool("v", false, "verbose output")
	jsonOutputFlag := flag.Bool("j", false, "json output")
	skipTLSFlag := flag.Bool("insecure", false, "skip TLS verification")
	intervalFlag := flag.Duration("interval", 10*time.Minute, "detection interval")
	flag.Parse()

	return *versionFlag, *verboseFlag, *jsonOutputFlag, *skipTLSFlag, *intervalFlag
}

func main() {
	versionFlag, verboseFlag, jsonOutputFlag, skipTLSFlag, interval := parseFlags()

	if versionFlag {
		fmt.Printf("%s %s (%s)\n", os.Args[0], build.LinkerVersion, build.LinkerCommitSHA)
		os.Exit(0)
	}
	log := newLogger(verboseFlag, jsonOutputFlag)

	token, err := getToken()
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	grafanaURL := grafana.DefaultBaseURL
	if flag.NArg() >= 1 {
		grafanaURL = flag.Arg(0)
	}

	opts := []api.ClientOption{api.WithAuthentication(token)}
	if skipTLSFlag {
		opts = append(opts, api.WithHTTPClient(&http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}))
	}
	client := grafana.NewAPIClient(api.NewClient(grafanaURL, opts...))

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		log.Log("Received shutdown signal")
		cancel()
		ticker.Stop()
	}()

	// Run detection on startup
	runDetection(ctx, log, client, jsonOutputFlag)

	// Run detection periodically
	log.Log("Starting periodic detection loop with interval %s", interval)
	for {
		select {
		case <-ctx.Done():
			log.Log("Shutting down")
			return
		case <-ticker.C:
			runDetection(ctx, log, client, jsonOutputFlag)
		}
	}
}
