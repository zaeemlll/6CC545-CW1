package model

import (
	"strings"
	"testing"
)

func TestWeek03_MetadataValidation(t *testing.T) {
	good := FileMetadata{Name: "notes.txt", Size: 1, SHA256: strings.Repeat("0", 64), ModifiedUnix: 1}
	if e := good.Validate(); e != nil {
		t.Fatal(e)
	}
	cases := []FileMetadata{good, good, good, good}
	cases[0].Name = "../escape"
	cases[1].Size = -1
	cases[2].SHA256 = "not-a-hash"
	cases[3].ModifiedUnix = -1
	for _, f := range cases {
		if f.Validate() == nil {
			t.Fatal("invalid metadata accepted")
		}
	}
}

func TestPeerValidation(t *testing.T) {
	good := Peer{
		ID:          "node-b",
		HTTPAddress: "127.0.0.1:8082",
		GRPCAddress: "127.0.0.1:9092",
		Status:      "unknown",
	}

	for _, status := range []string{"unknown", "online", "offline"} {
		p := good
		p.Status = status
		if err := p.Validate(); err != nil {
			t.Errorf("valid status %q rejected: %v", status, err)
		}
	}

	securePeer := good
	securePeer.HTTPAddress = "https://node-b:9444"
	if err := securePeer.Validate(); err != nil {
		t.Errorf("valid HTTPS peer address rejected: %v", err)
	}

	cases := []struct {
		name string
		peer Peer
	}{
		{"empty ID", func() Peer { p := good; p.ID = ""; return p }()},
		{"invalid HTTP address", func() Peer { p := good; p.HTTPAddress = "broken"; return p }()},
		{"insecure HTTP URL", func() Peer { p := good; p.HTTPAddress = "http://node-b:9444"; return p }()},
		{"HTTPS URL missing port", func() Peer { p := good; p.HTTPAddress = "https://node-b"; return p }()},
		{"HTTPS URL with path", func() Peer { p := good; p.HTTPAddress = "https://node-b:9444/files"; return p }()},
		{"invalid gRPC address", func() Peer { p := good; p.GRPCAddress = "broken"; return p }()},
		{"invalid status", func() Peer { p := good; p.Status = "sleeping"; return p }()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.peer.Validate(); err == nil {
				t.Fatal("invalid peer accepted")
			}
		})
	}
}

func TestWeek03_MetadataValidationBoundaries(t *testing.T) {
	good := FileMetadata{
		Name:         "notes.txt",
		Size:         0,
		SHA256:       strings.Repeat("a", 64),
		ModifiedUnix: 0,
	}

	if err := good.Validate(); err != nil {
		t.Fatalf("zero size and modification time should be valid: %v", err)
	}

	cases := []struct {
		name string
		file FileMetadata
	}{
		{"empty name", func() FileMetadata { f := good; f.Name = ""; return f }()},
		{"dot name", func() FileMetadata { f := good; f.Name = "."; return f }()},
		{"double dot name", func() FileMetadata { f := good; f.Name = ".."; return f }()},
		{"forward slash", func() FileMetadata { f := good; f.Name = "folder/file.txt"; return f }()},
		{"backslash", func() FileMetadata { f := good; f.Name = `folder\file.txt`; return f }()},
		{"colon", func() FileMetadata { f := good; f.Name = "file:name.txt"; return f }()},
		{"null character", func() FileMetadata { f := good; f.Name = "file\x00.txt"; return f }()},
		{"name too long", func() FileMetadata { f := good; f.Name = strings.Repeat("a", 129); return f }()},
		{"empty hash", func() FileMetadata { f := good; f.SHA256 = ""; return f }()},
		{"invalid hexadecimal hash", func() FileMetadata { f := good; f.SHA256 = strings.Repeat("g", 64); return f }()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.file.Validate(); err == nil {
				t.Fatal("invalid metadata accepted")
			}
		})
	}
}
