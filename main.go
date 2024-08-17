package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/grafana/detect-angular-dashboards/api"
	"github.com/grafana/detect-angular-dashboards/api/gcom"
	"github.com/grafana/detect-angular-dashboards/api/grafana"
	"github.com/grafana/detect-angular-dashboards/build"
	"github.com/grafana/detect-angular-dashboards/detector"
	"github.com/grafana/detect-angular-dashboards/flags"
	"github.com/grafana/detect-angular-dashboards/logger"
	"github.com/grafana/detect-angular-dashboards/output"
)

const envGrafana = "GRAFANA_TOKEN"

func main() {
	flags := flags.ParseFlags()

	if flags.Version {
		fmt.Printf("%s %s (%s)\n", os.Args[0], build.LinkerVersion, build.LinkerCommitSHA)
		os.Exit(0)
	}
	log := newLogger(flags.Verbose, flags.JSONOutput)

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
	if flags.SkipTLS {
		opts = append(opts, api.WithHTTPClient(&http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}))
	}
	client := grafana.NewAPIClient(api.NewClient(grafanaURL, opts...))

	ticker := time.NewTicker(flags.Interval)
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

	// Channel used to send runDetection results to the HTTP server
	outputChan := make(chan []output.Dashboard)
	var outputData []output.Dashboard
	var mu sync.Mutex

	go func() {
		for data := range outputChan {
			log.Log("Updating outputData with results from most recent detection")
			mu.Lock()
			outputData = data
			mu.Unlock()
		}
	}()

	// Run detection on startup
	runDetection(ctx, log, client, outputChan)

	if flags.ServerMode {
		// Run detection periodically
		log.Log("Starting periodic detection loop with interval %s", flags.Interval)
		go func() {
			http.HandleFunc("/output", func(w http.ResponseWriter, r *http.Request) {
				handleOutputRequest(w, r, &mu, outputData, log)
			})
			log.Log("Listening on :8080")
			if err := http.ListenAndServe(":8080", nil); err != nil {
				log.Errorf("http server: %s\n", err)
			}
		}()

		for {
			select {
			case <-ctx.Done():
				log.Log("Shutting down")
				return
			case <-ticker.C:
				runDetection(ctx, log, client, outputChan)
			}
		}
	}
}

// handleOutputRequest handles the /output HTTP endpoint.
func handleOutputRequest(w http.ResponseWriter, r *http.Request, mu *sync.Mutex, outputData []output.Dashboard, log *logger.LeveledLogger) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mu.Lock()
	defer mu.Unlock()
	w.Header().Set("Content-Type", "application/json")

	// Filter Angular dashboards to avoid modifying the original slice
	angularDashboards := filterAngularDashboards(outputData)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	if err := enc.Encode(angularDashboards); err != nil {
		log.Errorf("http server: %s\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// filterAngularDashboards filters dashboards to include only those with detections.
func filterAngularDashboards(dashboards []output.Dashboard) []output.Dashboard {
	var angularDashboards []output.Dashboard
	for _, dashboard := range dashboards {
		if len(dashboard.Detections) > 0 {
			angularDashboards = append(angularDashboards, dashboard)
		}
	}
	return angularDashboards
}

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

// getToken retrieves the Grafana token from the environment.
func getToken() (string, error) {
	token := os.Getenv(envGrafana)
	if token == "" {
		return "", fmt.Errorf("environment variable %s is not set", envGrafana)
	}
	return token, nil
}

// runDetection performs the detection of Angular dashboards and sends the output to a channel.
func runDetection(ctx context.Context, log *logger.LeveledLogger, client grafana.APIClient, outputChan chan<- []output.Dashboard) {
	log.Log("Detecting Angular dashboards")

	d := detector.NewDetector(log, client, gcom.NewAPIClient())
	finalOutput, err := d.Run(ctx)
	if err != nil {
		log.Errorf("%s\n", err)
		return
	}

	// Send output to channel
	outputChan <- finalOutput
}
