package flags

import (
	"flag"
	"time"
)

// Flags holds the command-line flags.
type Flags struct {
	Version        bool
	Verbose        bool
	JSONOutput     bool
	SkipTLS        bool
	Server         string
	Interval       time.Duration
	MaxConcurrency int
}

// Parse parses the command-line flags.
func Parse() Flags {
	var flags Flags
	flag.BoolVar(&flags.Version, "version", false, "print version number")
	flag.BoolVar(&flags.Verbose, "v", false, "verbose output")
	flag.BoolVar(&flags.JSONOutput, "j", false, "json output")
	flag.BoolVar(&flags.SkipTLS, "insecure", false, "skip TLS verification")
	flag.DurationVar(&flags.Interval, "interval", 5*time.Minute, "detection refresh interval when running in HTTP server mode")
	flag.StringVar(&flags.Server, "server", "", "Run as HTTP server instead of CLI. Value must be a listen address (e.g.: 0.0.0.0:5000. Output is exposed as JSON at /detections.")
	flag.IntVar(&flags.MaxConcurrency, "max-concurrency", 10, "maximum number of concurrent dashboard downloads")
	flag.Parse()

	return flags
}
