package config

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Config is process-local. Ports must differ between simultaneously running nodes.
type Config struct {
	ID            string
	HTTP          string
	TCP           string
	UDP           string
	GRPC          string
	Seed          string
	Timeout       time.Duration
	HTTPAdvertise string
	GRPCAdvertise string
	DataDir       string
	PeerSeeds     []string
}

// Parse reads command-line arguments and validates the resulting configuration.
// Precedence: explicit command-line flags, environment variables, local defaults.
func Parse(args []string, out io.Writer) (Config, error) {
	c := Config{
		ID:      "node-a",
		HTTP:    "127.0.0.1:8081",
		Timeout: 2 * time.Second,
	}

	// Environment variables override local defaults.
	envString("NODE_ID", &c.ID)
	envString("HTTP_ADDR", &c.HTTP)
	envString("GRPC_ADDR", &c.GRPC)
	envString("HTTP_ADVERTISE", &c.HTTPAdvertise)
	envString("GRPC_ADVERTISE", &c.GRPCAdvertise)
	envString("DATA_DIR", &c.DataDir)

	if seeds, ok := os.LookupEnv("PEER_SEEDS"); ok {
		c.PeerSeeds = parsePeerSeeds(seeds)
	}

	f := flag.NewFlagSet("node", flag.ContinueOnError)
	f.SetOutput(out)

	f.StringVar(&c.ID, "id", c.ID, "unique local node ID")
	f.StringVar(&c.HTTP, "http", c.HTTP, "HTTP listen address")
	f.StringVar(&c.TCP, "tcp", "", "Week 4: TCP address; empty disables")
	f.StringVar(&c.UDP, "udp", "", "Week 4: UDP address; empty disables")
	f.StringVar(&c.GRPC, "grpc", c.GRPC, "gRPC listen address; empty disables")
	f.StringVar(&c.Seed, "seed", "", "Week 3: synthetic metadata JSON file")
	f.DurationVar(&c.Timeout, "timeout", c.Timeout, "network operation timeout")
	f.StringVar(&c.HTTPAdvertise, "http-advertise", c.HTTPAdvertise, "advertised HTTP URL")
	f.StringVar(&c.GRPCAdvertise, "grpc-advertise", c.GRPCAdvertise, "advertised gRPC host and port")
	f.StringVar(&c.DataDir, "data-dir", c.DataDir, "file storage directory")

	peerSeeds := strings.Join(c.PeerSeeds, ",")
	f.StringVar(&peerSeeds, "peer-seeds", peerSeeds, "comma-separated peer gRPC addresses")

	if err := f.Parse(args); err != nil {
		return c, err
	}
	if len(f.Args()) != 0 {
		return c, fmt.Errorf("unexpected arguments: %v", f.Args())
	}

	c.PeerSeeds = parsePeerSeeds(peerSeeds)

	return c, c.Validate()
}

func envString(name string, target *string) {
	if value, ok := os.LookupEnv(name); ok {
		*target = value
	}
}

func parsePeerSeeds(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	seeds := make([]string, 0, len(parts))
	for _, part := range parts {
		seed := strings.TrimSpace(part)
		if seed != "" {
			seeds = append(seeds, seed)
		}
	}
	return seeds
}

// ValidID and ValidAddress are supplied helpers reused by later models.
var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

func ValidID(s string) bool {
	return idPattern.MatchString(s)
}

// ValidAddress retains the original local-mode loopback-only policy.
func ValidAddress(s string) bool {
	h, p, err := net.SplitHostPort(s)
	if err != nil || h == "" {
		return false
	}
	n, err := strconv.Atoi(p)
	if err != nil || n < 1 || n > 65535 {
		return false
	}
	return h == "localhost" || (net.ParseIP(h) != nil && net.ParseIP(h).IsLoopback())
}

// validListenAddress permits loopback addresses and the unspecified address
// used inside a container. It does not permit arbitrary remote interfaces.
func validListenAddress(s string) bool {
	if ValidAddress(s) {
		return true
	}

	h, p, err := net.SplitHostPort(s)
	if err != nil || h == "" {
		return false
	}
	n, err := strconv.Atoi(p)
	if err != nil || n < 1 || n > 65535 {
		return false
	}

	ip := net.ParseIP(h)
	return ip != nil && ip.IsUnspecified()
}

// ValidPeerAddress validates a host:port endpoint, including lab service DNS.
func ValidPeerAddress(s string) bool {
	h, p, err := net.SplitHostPort(s)
	if err != nil || !validPort(p) || h == "" {
		return false
	}

	if ip := net.ParseIP(h); ip != nil {
		return !ip.IsUnspecified()
	}

	if h == "localhost" {
		return true
	}

	if len(h) > 253 || strings.HasPrefix(h, ".") || strings.HasSuffix(h, ".") {
		return false
	}

	for _, label := range strings.Split(h, ".") {
		if len(label) == 0 || len(label) > 63 ||
			label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z') &&
				!(r >= 'A' && r <= 'Z') &&
				!(r >= '0' && r <= '9') && r != '-' {
				return false
			}
		}
	}
	return true
}

func validPort(port string) bool {
	n, err := strconv.Atoi(port)
	return err == nil && n >= 1 && n <= 65535
}

func validHTTPAdvertise(value string) bool {
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "http" || u.Host == "" ||
		u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	if u.Path != "" && u.Path != "/" {
		return false
	}
	return ValidPeerAddress(u.Host)
}

// Validate checks configuration without opening any network sockets.
func (c Config) Validate() error {
	if !ValidID(c.ID) {
		return fmt.Errorf("invalid node ID %q: use 1-32 letters, numbers, underscores or hyphens", c.ID)
	}
	validHTTP := ValidAddress(c.HTTP)
	if !validHTTP && c.HTTPAdvertise != "" &&
		validHTTPAdvertise(c.HTTPAdvertise) {
		validHTTP = validListenAddress(c.HTTP)
	}
	if !validHTTP {
		return fmt.Errorf("invalid HTTP address %q: use a loopback address or a container listen address with a valid advertised HTTP URL", c.HTTP)
	}

	if c.Timeout <= 0 || c.Timeout > 30*time.Second {
		return fmt.Errorf("invalid timeout %s: must be greater than 0 and no more than 30s", c.Timeout)
	}

	for name, address := range map[string]string{
		"TCP": c.TCP,
		"UDP": c.UDP,
	} {
		if address != "" && !ValidAddress(address) {
			return fmt.Errorf("invalid optional %s address %q: leave it empty or use a loopback host and port 1-65535", name, address)
		}
	}

	if c.GRPC != "" {
		validGRPC := ValidAddress(c.GRPC)
		if !validGRPC && c.GRPCAdvertise != "" &&
			ValidPeerAddress(c.GRPCAdvertise) {
			validGRPC = validListenAddress(c.GRPC)
		}
		if !validGRPC {
			return fmt.Errorf("invalid gRPC address %q: use a loopback address or a container listen address with a valid advertised gRPC address", c.GRPC)
		}
	}

	if c.HTTPAdvertise != "" {
		if !validHTTPAdvertise(c.HTTPAdvertise) {
			return fmt.Errorf("invalid advertised HTTP URL %q", c.HTTPAdvertise)
		}

		advertisedURL, _ := url.Parse(c.HTTPAdvertise)
		if advertisedURL.Hostname() != c.ID {
			return fmt.Errorf("advertised HTTP hostname %q does not match node ID %q",
				advertisedURL.Hostname(), c.ID)
		}
	}

	if c.GRPCAdvertise != "" {
		if !ValidPeerAddress(c.GRPCAdvertise) {
			return fmt.Errorf("invalid advertised gRPC address %q", c.GRPCAdvertise)
		}

		advertisedHost, _, _ := net.SplitHostPort(c.GRPCAdvertise)
		if advertisedHost != c.ID {
			return fmt.Errorf("advertised gRPC hostname %q does not match node ID %q",
				advertisedHost, c.ID)
		}
	}

	if c.DataDir != "" {
		if strings.ContainsRune(c.DataDir, 0) || !filepath.IsAbs(c.DataDir) {
			return fmt.Errorf("invalid data directory %q: use an absolute path", c.DataDir)
		}
	}

	for _, seed := range c.PeerSeeds {
		if !ValidPeerAddress(seed) {
			return fmt.Errorf("invalid peer seed %q: use a valid host and port", seed)
		}
	}

	return nil
}
