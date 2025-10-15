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
	"sync/atomic"
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

type Output struct {
	mu   sync.Mutex
	data []output.Dashboard
}

func main() {
	f := flags.Parse()

	if f.Version {
		fmt.Printf("%s %s (%s)\n", os.Args[0], build.LinkerVersion, build.LinkerCommitSHA)
		os.Exit(0)
	}
	log := newLogger(f.Verbose, f.JSONOutput)

	token, err := getToken()
	if err != nil {
		log.Errorf("Failed to retrieve Grafana token: %s\n", err.Error())
		os.Exit(1)
	}
	client := initializeClient(token, &f)

	d := detector.NewDetector(log, client, gcom.NewAPIClient(), f.MaxConcurrency)

	if f.Server != "" {
		if err := runServerMode(&f, log, d); err != nil {
			log.Errorf("%s\n", err)
			os.Exit(1)
		}
		return
	}

	if err := runCLIMode(&f, log, d); err != nil {
		log.Errorf("%s\n", err)
		os.Exit(1)
	}
}

// runServerMode runs the program in server (HTTP) mode.
func runServerMode(flags *flags.Flags, log *logger.LeveledLogger, d *detector.Detector) error {
	// Readiness flag using atomic boolean
	var ready atomic.Bool
	var once sync.Once

	ticker := time.NewTicker(flags.Interval)
	defer ticker.Stop()
	log.Log("Running detection every %s", flags.Interval)

	var out Output
	go func() {
		// Trigger for the first time
		run := make(chan struct{}, 1)
		run <- struct{}{}

		for {
			select {
			case <-run:
			case <-ticker.C:
			}

			// Run detection periodically
			log.Log("Detecting Angular dashboards")
			data, err := d.Run(context.Background())
			if err != nil {
				log.Errorf("%s\n", err)
				continue
			}

			// Run detection periodically
			log.Log("Updating Output Data")
			out.mu.Lock()
			out.data = data
			out.mu.Unlock()

			// Use sync.Once to set readiness only once
			once.Do(func() {
				ready.Store(true)
				log.Log("Updating readiness probe to ready")
			})
		}
	}()

	http.HandleFunc("/detections", func(w http.ResponseWriter, r *http.Request) {
		handleDetectionsRequest(w, r, &out, log)
	})
	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		handleReadyRequest(w, r, &ready)
	})

	if err := runServer(flags, log); err != nil {
		log.Error("runServer Failed with the following err: %v", err)
		return err
	}

	return nil
}

func runServer(flags *flags.Flags, log *logger.LeveledLogger) error {
	server := &http.Server{Addr: flags.Server}

	// Channel to listen for OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the server in a goroutine
	go func() {
		log.Log("Listening on %s", flags.Server)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("ListenAndServe(): %s", err)
		}
	}()

	// Wait for a signal interrupt
	sig := <-sigChan
	log.Log("Received signal: %s. Shutting down server...", sig)

	// Gracefully shut down the server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Error("Server Shutdown Failed:%+v", err)
		return err
	}
	log.Log("Server gracefully stopped")

	return nil
}

// runCLIMode runs the program in CLI mode.
func runCLIMode(flags *flags.Flags, log *logger.LeveledLogger, d *detector.Detector) error {
	log.Log("Detecting Angular dashboards")
	var out output.Outputter
	if flags.JSONOutput {
		out = output.NewJSONOutputter(os.Stdout)
	} else {
		out = output.NewLoggerReadableOutput(log)
	}
	data, err := d.Run(context.Background())
	if err != nil {
		return fmt.Errorf("run detector: %w", err)
	}
	if err := out.Output(data); err != nil {
		return fmt.Errorf("output: %w", err)
	}
	return nil
}

// initializeClient initializes the Grafana API client.
func initializeClient(token string, flags *flags.Flags) grafana.APIClient {
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
	return grafana.NewAPIClient(api.NewClient(grafanaURL, opts...))
}

// handleDetectionsRequest handles the /output HTTP endpoint.
func handleDetectionsRequest(w http.ResponseWriter, r *http.Request, output *Output, log *logger.LeveledLogger) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	output.mu.Lock()
	defer output.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")

	// Have to do this because the JSONOutputter.Output method modifies the slice in place
	// which results in werid bug where the slice gets duplicate entries. The number of duplicate entries
	// continues to grow with each request to /output. Something is leaky
	angularDashboards := filterAngularDashboards(output.data)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	if err := enc.Encode(angularDashboards); err != nil {
		log.Errorf("http server: %s\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleReadyRequest handles the /ready HTTP endpoint.
func handleReadyRequest(w http.ResponseWriter, r *http.Request, ready *atomic.Bool) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if ready.Load() {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Ready"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("Not Ready"))
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
