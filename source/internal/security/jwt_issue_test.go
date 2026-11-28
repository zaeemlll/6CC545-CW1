package security

import (
	"testing"
	"time"
)

func TestIssueReaderToken(t *testing.T) {
	now := time.Unix(1800000000, 0)
	validator := TokenValidator{
		Key:      []byte("local-test-only-32-byte-randomness-not-for-deployment"),
		Issuer:   "cw2-lab",
		Audience: "cw2-api",
		Now:      func() time.Time { return now },
	}

	raw, err := validator.IssueReaderToken("node-a", 30*time.Second)
	if err != nil {
		t.Fatalf("issue reader token: %v", err)
	}

	claims, err := validator.Validate(raw, "reader")
	if err != nil {
		t.Fatalf("validate reader token: %v", err)
	}
	if claims.Subject != "node-a" || claims.Role != "reader" {
		t.Fatalf("unexpected claims: subject=%q role=%q", claims.Subject, claims.Role)
	}
	if !claims.ExpiresAt.Time.Equal(now.Add(30 * time.Second)) {
		t.Fatalf("unexpected expiry: %v", claims.ExpiresAt.Time)
	}
	if _, err := validator.Validate(raw, "writer"); err == nil {
		t.Fatal("reader token was accepted for writer access")
	}
}

func TestIssueReaderTokenRejectsInvalidInput(t *testing.T) {
	valid := TokenValidator{
		Key:      []byte("local-test-only-32-byte-randomness-not-for-deployment"),
		Issuer:   "cw2-lab",
		Audience: "cw2-api",
	}

	tests := []struct {
		name      string
		validator TokenValidator
		subject   string
		ttl       time.Duration
	}{
		{"short key", TokenValidator{Key: []byte("short"), Issuer: "cw2-lab", Audience: "cw2-api"}, "node-a", time.Second},
		{"missing issuer", TokenValidator{Key: valid.Key, Audience: "cw2-api"}, "node-a", time.Second},
		{"missing audience", TokenValidator{Key: valid.Key, Issuer: "cw2-lab"}, "node-a", time.Second},
		{"empty subject", valid, "", time.Second},
		{"zero lifetime", valid, "node-a", 0},
		{"negative lifetime", valid, "node-a", -time.Second},
		{"excessive lifetime", valid, "node-a", time.Minute + time.Second},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.validator.IssueReaderToken(tc.subject, tc.ttl); err == nil {
				t.Fatal("expected token issuance to fail")
			}
		})
	}
}
