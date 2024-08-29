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
	flags := flags.ParseFlags()

	if flags.Version {
		fmt.Printf("%s %s (%s)\n", os.Args[0], build.LinkerVersion, build.LinkerCommitSHA)
		os.Exit(0)
	}
	log := newLogger(flags.Verbose, flags.JSONOutput)

	token, err := getToken()
	if err != nil {
		log.Errorf("Failed to retrieve Grafana token: %s\n", err.Error())
		os.Exit(1)
	}
	client := initializeClient(token, &flags)

	d := detector.NewDetector(log, client, gcom.NewAPIClient(), flags.MaxConcurrency)

	if flags.ServerMode {
		// Readiness flag using atomic boolean
		var ready int32
		var once sync.Once

		ticker := time.NewTicker(flags.Interval)
		defer ticker.Stop()
		run := make(chan struct{}, 1)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle graceful shutdown
		// setupGracefulShutdown(cancel, ticker, log)

		var out Output
		go func() {
			run <- struct{}{}
			for {
				select {
				case <-ctx.Done():
					log.Log("Shutting down")
					return
				case <-run:
				case <-ticker.C:
				}

				// Run detection periodically
				log.Log("Running detection")
				data, err := d.Run(ctx)
				if err != nil {
					log.Errorf("%s\n", err)
					continue
				}

				out.mu.Lock()
				out.data = data
				out.mu.Unlock()

				// Use sync.Once to set readiness only once
				once.Do(func() {
					atomic.StoreInt32(&ready, 1)
					log.Log("Updating readiness probe to ready")
				})
			}
		}()

		http.HandleFunc("/output", func(w http.ResponseWriter, r *http.Request) {
			handleOutputRequest(w, r, &out, log)
		})
		http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
			handleReadyRequest(w, r, &ready)
		})

		serverAddress := fmt.Sprintf(":%d", flags.ServerPort)
		log.Log("Listening on %s", serverAddress)
		if err := http.ListenAndServe(serverAddress, nil); err != nil {
			log.Errorf("Failed to setup http server: %s\n", err)
			os.Exit(1)
		}
	} else {
		var out output.Outputter
		if flags.JSONOutput {
			out = output.NewJSONOutputter(os.Stdout)
		} else {
			out = output.NewLoggerReadableOutput(log)
		}
		// Print output
		data, err := d.Run(context.Background())
		if err != nil {
			log.Errorf("%s\n", err)
			os.Exit(1)
		}
		if err := out.Output(data); err != nil {
			log.Errorf("output: %s\n", err)
		}
	}
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

// setupGracefulShutdown sets up a signal handler for SIGINT and SIGTERM to gracefully shutdown the application.
func setupGracefulShutdown(cancel context.CancelFunc, ticker *time.Ticker, log *logger.LeveledLogger) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		log.Log("Received shutdown signal")
		cancel()
		ticker.Stop()
	}()
}

// handleOutputRequest handles the /output HTTP endpoint.
func handleOutputRequest(w http.ResponseWriter, r *http.Request, output *Output, log *logger.LeveledLogger) {
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
func handleReadyRequest(w http.ResponseWriter, r *http.Request, ready *int32) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if atomic.LoadInt32(ready) == 1 {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Ready"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Not Ready"))
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
