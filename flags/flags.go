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
	flag.DurationVar(&flags.Interval, "interval", 5*time.Minute, "detection refresh interval")
	flag.StringVar(&flags.Server, "server", "", "Run as http server instead of CLI. Output is exposed as JSON at /output. Default refersh interval is 5 minutes.")
	flag.IntVar(&flags.MaxConcurrency, "max-concurrency", 10, "maximum number of concurrent dashboard downloads")
	flag.Parse()

	return flags
}
