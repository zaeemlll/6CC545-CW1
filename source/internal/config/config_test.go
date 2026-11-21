package config

import (
	"io"
	"testing"
	"time"
)

func TestSmoke_ParseHelp(t *testing.T) {
	if _, e := Parse([]string{"-help"}, io.Discard); e == nil {
		t.Fatal("help must return flag.ErrHelp")
	}
}
func TestWeek01_Config(t *testing.T) {
	good := Config{ID: "node-a", HTTP: "127.0.0.1:8081", Timeout: 2 * time.Second}
	if e := good.Validate(); e != nil {
		t.Fatalf("valid configuration rejected: %v", e)
	}
	cases := []Config{good, good, good, good, good}
	cases[0].ID = ""
	cases[1].HTTP = ":8081"
	cases[2].HTTP = "127.0.0.1:0"
	cases[3].Timeout = 0
	cases[4].HTTP = "0.0.0.0:8081"
	for i, c := range cases {
		if c.Validate() == nil {
			t.Errorf("invalid case %d accepted", i)
		}
	}
}
func TestWeek01_UnknownFlag(t *testing.T) {
	if _, e := Parse([]string{"-bogus"}, io.Discard); e == nil {
		t.Fatal("unknown flag accepted")
	}
}

func TestWeek08_DeploymentEnvironment(t *testing.T) {
	t.Setenv("NODE_ID", "node-a")
	t.Setenv("HTTP_ADDR", "0.0.0.0:8080")
	t.Setenv("GRPC_ADDR", "0.0.0.0:9090")
	t.Setenv("HTTP_ADVERTISE", "http://node-a:8080")
	t.Setenv("GRPC_ADVERTISE", "node-a:9090")
	t.Setenv("DATA_DIR", "/data")
	t.Setenv("PEER_SEEDS", "node-b:9090,node-c:9090")

	c, err := Parse(nil, io.Discard)
	if err != nil {
		t.Fatalf("deployment configuration rejected: %v", err)
	}

	if c.ID != "node-a" ||
		c.HTTP != "0.0.0.0:8080" ||
		c.GRPC != "0.0.0.0:9090" ||
		c.HTTPAdvertise != "http://node-a:8080" ||
		c.GRPCAdvertise != "node-a:9090" ||
		c.DataDir != "/data" {
		t.Fatalf("unexpected deployment configuration: %+v", c)
	}

	if len(c.PeerSeeds) != 2 ||
		c.PeerSeeds[0] != "node-b:9090" ||
		c.PeerSeeds[1] != "node-c:9090" {
		t.Fatalf("unexpected peer seeds: %v", c.PeerSeeds)
	}
}

func TestWeek08_CommandLineOverridesEnvironment(t *testing.T) {
	t.Setenv("NODE_ID", "node-from-env")
	t.Setenv("HTTP_ADDR", "127.0.0.1:8081")

	c, err := Parse([]string{"-id", "node-from-flag"}, io.Discard)
	if err != nil {
		t.Fatalf("configuration rejected: %v", err)
	}

	if c.ID != "node-from-flag" {
		t.Fatalf("command-line flag did not override environment: got %q", c.ID)
	}
}

func TestWeek08_ValidPeerAddress(t *testing.T) {
	valid := []string{
		"node-a:9090",
		"node-b:9090",
		"localhost:9090",
		"127.0.0.1:9090",
	}

	for _, address := range valid {
		if !ValidPeerAddress(address) {
			t.Errorf("valid peer address rejected: %q", address)
		}
	}

	invalid := []string{
		"",
		"node-a",
		"node-a:0",
		"node-a:65536",
		"0.0.0.0:9090",
		"bad host:9090",
	}

	for _, address := range invalid {
		if ValidPeerAddress(address) {
			t.Errorf("invalid peer address accepted: %q", address)
		}
	}
}

func TestWeek08_NonLoopbackListenersRequireAdvertiseAddresses(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		wantOK bool
	}{
		{
			name: "data directory alone is insufficient",
			config: Config{
				ID:      "node-a",
				HTTP:    "0.0.0.0:8080",
				DataDir: "/data",
				Timeout: 2 * time.Second,
			},
		},
		{
			name: "HTTP requires advertised address",
			config: Config{
				ID:      "node-a",
				HTTP:    "0.0.0.0:8080",
				GRPC:    "127.0.0.1:9090",
				Timeout: 2 * time.Second,
			},
		},
		{
			name: "valid HTTP advertised address allows HTTP listener",
			config: Config{
				ID:            "node-a",
				HTTP:          "0.0.0.0:8080",
				GRPC:          "127.0.0.1:9090",
				HTTPAdvertise: "http://node-a:8080",
				Timeout:       2 * time.Second,
			},
			wantOK: true,
		},
		{
			name: "gRPC requires advertised address",
			config: Config{
				ID:      "node-a",
				HTTP:    "127.0.0.1:8080",
				GRPC:    "0.0.0.0:9090",
				Timeout: 2 * time.Second,
			},
		},
		{
			name: "valid gRPC advertised address allows gRPC listener",
			config: Config{
				ID:            "node-a",
				HTTP:          "127.0.0.1:8080",
				GRPC:          "0.0.0.0:9090",
				GRPCAdvertise: "node-a:9090",
				Timeout:       2 * time.Second,
			},
			wantOK: true,
		},
		{
			name: "both advertised addresses allow deployment",
			config: Config{
				ID:            "node-a",
				HTTP:          "0.0.0.0:8080",
				GRPC:          "0.0.0.0:9090",
				HTTPAdvertise: "http://node-a:8080",
				GRPCAdvertise: "node-a:9090",
				DataDir:       "/data",
				Timeout:       2 * time.Second,
			},
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantOK && err != nil {
				t.Fatalf("expected valid configuration, got: %v", err)
			}
			if !tt.wantOK && err == nil {
				t.Fatal("expected invalid configuration, but it was accepted")
			}
		})
	}
}

func TestWeek08_AdvertisedHostnamesMustMatchNodeID(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "mismatched HTTP hostname",
			config: Config{
				ID:            "node-a",
				HTTP:          "0.0.0.0:8080",
				HTTPAdvertise: "http://node-b:8080",
				GRPC:          "127.0.0.1:9090",
				Timeout:       2 * time.Second,
			},
		},
		{
			name: "mismatched gRPC hostname",
			config: Config{
				ID:            "node-a",
				HTTP:          "127.0.0.1:8080",
				GRPC:          "0.0.0.0:9090",
				GRPCAdvertise: "node-b:9090",
				Timeout:       2 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.config.Validate(); err == nil {
				t.Fatal("expected mismatched advertised hostname to be rejected")
			}
		})
	}
}
