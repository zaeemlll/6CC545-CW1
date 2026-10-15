package config

import (
	"flag"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"time"
)

// Config is process-local. Ports must differ between simultaneously running nodes.
type Config struct {
	ID      string
	HTTP    string
	TCP     string
	UDP     string
	GRPC    string
	Seed    string
	Timeout time.Duration
}

// Parse reads command-line arguments and validates the resulting configuration.
func Parse(args []string, out io.Writer) (Config, error) {
	var c Config
	f := flag.NewFlagSet("node", flag.ContinueOnError)
	f.SetOutput(out)
	f.StringVar(&c.ID, "id", "node-a", "unique local node ID")
	f.StringVar(&c.HTTP, "http", "127.0.0.1:8081", "HTTP listen address")
	f.StringVar(&c.TCP, "tcp", "", "Week 4: TCP address; empty disables")
	f.StringVar(&c.UDP, "udp", "", "Week 4: UDP address; empty disables")
	f.StringVar(&c.GRPC, "grpc", "", "Week 5: gRPC address; empty disables")
	f.StringVar(&c.Seed, "seed", "", "Week 3: synthetic metadata JSON file")
	f.DurationVar(&c.Timeout, "timeout", 2*time.Second, "network operation timeout")

	if err := f.Parse(args); err != nil {
		return c, err
	}
	if len(f.Args()) != 0 {
		return c, fmt.Errorf("unexpected arguments: %v", f.Args())
	}
	return c, c.Validate()
}

// ValidID and ValidAddress are supplied helpers reused by later models.
var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

func ValidID(s string) bool { return idPattern.MatchString(s) }

func ValidAddress(s string) bool {
	h, p, err := net.SplitHostPort(s)
	if err != nil || h == "" {
		return false
	}
	n, err := strconv.Atoi(p)
	if err != nil || n < 1 || n > 65535 {
		return false
	}
	// Restrict this teaching scaffold to loopback. This is not authentication.
	return h == "localhost" || (net.ParseIP(h) != nil && net.ParseIP(h).IsLoopback())
}

// Validate checks configuration without opening any network sockets.
func (c Config) Validate() error {
	if !ValidID(c.ID) {
		return fmt.Errorf("invalid node ID %q: use 1-32 letters, numbers, underscores or hyphens", c.ID)
	}
	if !ValidAddress(c.HTTP) {
		return fmt.Errorf("invalid HTTP address %q: use a loopback host and port 1-65535", c.HTTP)
	}
	if c.Timeout <= 0 || c.Timeout > 30*time.Second {
		return fmt.Errorf("invalid timeout %s: must be greater than 0 and no more than 30s", c.Timeout)
	}

	for name, address := range map[string]string{
		"TCP":  c.TCP,
		"UDP":  c.UDP,
		"gRPC": c.GRPC,
	} {
		if address != "" && !ValidAddress(address) {
			return fmt.Errorf("invalid optional %s address %q: leave it empty or use a loopback host and port 1-65535", name, address)
		}
	}

	return nil
}
