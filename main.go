package main

import (
	"cmp"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strconv"

	"github.com/grafana/detect-angular-dashboards/api"
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
	skipTLSFlag := flag.Bool("insecure", false, "skip TLS verification")
	bulkDetectionFlag := flag.Bool("bulk", false, "detect use of angular in all orgs, requires basicauth instead of token")
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
	var (
		orgs       []grafana.Org
		currentOrg grafana.Org
	)

	log.Log("Detecting Angular dashboards for %q", grafanaURL)

	opts := []api.ClientOption{api.WithAuthentication(token)}
	if *skipTLSFlag {
		opts = append(opts, api.WithHTTPClient(&http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}))
	}
	client := grafana.NewAPIClient(api.NewClient(grafanaURL, opts...))
	currentOrg, err = client.GetCurrentOrg(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to get current org: %s\n", err)
		os.Exit(1)
	}

	// we can't do bulk detection with token
	if *bulkDetectionFlag && client.BasicAuthUser != "" {
		orgs, err = client.GetOrgs(ctx)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to get org: %s\n", err)
			os.Exit(1)
		}
		log.Log("Fount %d orgs to scan\n", len(orgs))
		slices.SortFunc(orgs, func(a, b grafana.Org) int {
			return cmp.Compare(a.ID, b.ID)
		})
	} else {
		orgs = append(orgs, currentOrg)
	}

	finalOutput := []output.Dashboard{}
	orgsFinalOutput := map[int][]output.Dashboard{}

	for _, org := range orgs {
		// we can only switch org with basicauth
		if client.BasicAuthUser != "" {
			log.Log("Detecting Angular dashboards for org: %s(%d)\n", org.Name, org.ID)
			err = client.UserSwitchContext(ctx, strconv.Itoa(org.ID))
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "failed to switch to org: %s\n", err)
				continue
			}
		}

		summary, err := detector.Run(ctx, log, client, org.ID)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to scan org %d: %s\n", org.ID, err)
			os.Exit(0)
		}
		orgsFinalOutput[org.ID] = summary
		finalOutput = append(finalOutput, summary...)
	}

	var out output.Outputter
	if *jsonOutputFlag {
		out = output.NewJSONOutputter(os.Stdout)
	} else {
		out = output.NewLoggerReadableOutput(log)
	}

	// Print output
	if *bulkDetectionFlag {
		if err := out.BulkOutput(orgsFinalOutput); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "output: %s\n", err)
		}
	} else {
		if err := out.Output(finalOutput); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "output: %s\n", err)
		}
	}

	// switch back to initial org
	if client.BasicAuthUser != "" {
		err = client.UserSwitchContext(ctx, strconv.Itoa(currentOrg.ID))
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to switch back to initial org: %s\n", err)
		}
	}
}
