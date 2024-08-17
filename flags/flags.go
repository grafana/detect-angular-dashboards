package flags

import (
	"flag"
	"time"
)

// Flags holds the command-line flags.
type Flags struct {
	Version    bool
	Verbose    bool
	JSONOutput bool
	SkipTLS    bool
	ServerMode bool
	ServerPort int
	Interval   time.Duration
}

// parseFlags parses the command-line flags.
func ParseFlags() Flags {
	var flags Flags
	flag.BoolVar(&flags.Version, "version", false, "print version number")
	flag.BoolVar(&flags.Verbose, "v", false, "verbose output")
	flag.BoolVar(&flags.JSONOutput, "j", false, "json output")
	flag.BoolVar(&flags.SkipTLS, "insecure", false, "skip TLS verification")
	flag.DurationVar(&flags.Interval, "interval", 10*time.Minute, "detection refresh interval")
	flag.BoolVar(&flags.ServerMode, "server", false, "Run as http server instead of CLI. Output is exposed as JSON at /output. Default port is 8080. Default refersh interval is 10 minutes.")
	flag.IntVar(&flags.ServerPort, "port", 8080, "Port to run the server on")
	flag.Parse()

	return flags
}
