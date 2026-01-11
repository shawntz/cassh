package ca

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// generateTestCAKey creates a test CA key pair
func generateTestCAKey(t *testing.T) []byte {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate CA key: %v", err)
	}

	block, err := ssh.MarshalPrivateKey(priv, "test CA key")
	if err != nil {
		t.Fatalf("Failed to marshal CA key: %v", err)
	}

	// Encode as PEM
	return pem.EncodeToMemory(block)
}

// generateTestUserKey creates a test user key pair
func generateTestUserKey(t *testing.T) (ssh.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate user key: %v", err)
	}

	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("Failed to convert to SSH public key: %v", err)
	}

	return sshPub, priv
}

func TestNewCA(t *testing.T) {
	caKey := generateTestCAKey(t)

	tests := []struct {
		name          string
		privateKey    []byte
		validityHours int
		principals    []string
		wantErr       bool
	}{
		{
			name:          "Valid CA creation",
			privateKey:    caKey,
			validityHours: 12,
			principals:    nil,
			wantErr:       false,
		},
		{
			name:          "Valid CA with principals",
			privateKey:    caKey,
			validityHours: 24,
			principals:    []string{"git", "deploy"},
			wantErr:       false,
		},
		{
			name:          "Invalid private key",
			privateKey:    []byte("invalid key"),
			validityHours: 12,
			principals:    nil,
			wantErr:       true,
		},
		{
			name:          "Empty private key",
			privateKey:    []byte{},
			validityHours: 12,
			principals:    nil,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ca, err := NewCA(tt.privateKey, tt.validityHours, tt.principals)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCA() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && ca == nil {
				t.Error("NewCA() returned nil CA without error")
			}
		})
	}
}

func TestSignPublicKey(t *testing.T) {
	caKey := generateTestCAKey(t)
	ca, err := NewCA(caKey, 12, nil)
	if err != nil {
		t.Fatalf("Failed to create CA: %v", err)
	}

	userPub, _ := generateTestUserKey(t)

	cert, err := ca.SignPublicKey(userPub, "test-key-id", "testuser")
	if err != nil {
		t.Fatalf("SignPublicKey() error = %v", err)
	}

	// Verify certificate properties
	if cert.KeyId != "test-key-id" {
		t.Errorf("KeyId = %q, want %q", cert.KeyId, "test-key-id")
	}

	if len(cert.ValidPrincipals) != 1 || cert.ValidPrincipals[0] != "testuser" {
		t.Errorf("ValidPrincipals = %v, want [testuser]", cert.ValidPrincipals)
	}

	if cert.CertType != ssh.UserCert {
		t.Errorf("CertType = %d, want %d (UserCert)", cert.CertType, ssh.UserCert)
	}

	// Verify validity period
	now := time.Now()
	validAfter := time.Unix(int64(cert.ValidAfter), 0)
	validBefore := time.Unix(int64(cert.ValidBefore), 0)

	if validAfter.After(now) {
		t.Errorf("ValidAfter (%v) is in the future", validAfter)
	}

	expectedExpiry := now.Add(12 * time.Hour)
	if validBefore.Before(expectedExpiry.Add(-time.Minute)) || validBefore.After(expectedExpiry.Add(time.Minute)) {
		t.Errorf("ValidBefore = %v, want approximately %v", validBefore, expectedExpiry)
	}

	// Verify extensions
	expectedExtensions := []string{
		"permit-agent-forwarding",
		"permit-port-forwarding",
		"permit-pty",
		"permit-user-rc",
	}
	for _, ext := range expectedExtensions {
		if _, ok := cert.Extensions[ext]; !ok {
			t.Errorf("Missing extension: %s", ext)
		}
	}
}

func TestSignPublicKeyWithPrincipals(t *testing.T) {
	caKey := generateTestCAKey(t)
	principals := []string{"git", "deploy", "admin"}
	ca, err := NewCA(caKey, 12, principals)
	if err != nil {
		t.Fatalf("Failed to create CA: %v", err)
	}

	userPub, _ := generateTestUserKey(t)

	cert, err := ca.SignPublicKey(userPub, "test-key-id", "testuser")
	if err != nil {
		t.Fatalf("SignPublicKey() error = %v", err)
	}

	// With configured principals, should use those instead of username
	if len(cert.ValidPrincipals) != len(principals) {
		t.Errorf("ValidPrincipals length = %d, want %d", len(cert.ValidPrincipals), len(principals))
	}
	for i, p := range principals {
		if cert.ValidPrincipals[i] != p {
			t.Errorf("ValidPrincipals[%d] = %q, want %q", i, cert.ValidPrincipals[i], p)
		}
	}
}

func TestGenerateKeyPair(t *testing.T) {
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	if pub == nil {
		t.Error("GenerateKeyPair() returned nil public key")
	}
	if priv == nil {
		t.Error("GenerateKeyPair() returned nil private key")
	}

	// Verify key sizes
	if len(pub) != ed25519.PublicKeySize {
		t.Errorf("Public key size = %d, want %d", len(pub), ed25519.PublicKeySize)
	}
	if len(priv) != ed25519.PrivateKeySize {
		t.Errorf("Private key size = %d, want %d", len(priv), ed25519.PrivateKeySize)
	}
}

func TestMarshalCertificate(t *testing.T) {
	caKey := generateTestCAKey(t)
	ca, err := NewCA(caKey, 12, nil)
	if err != nil {
		t.Fatalf("Failed to create CA: %v", err)
	}

	userPub, _ := generateTestUserKey(t)
	cert, err := ca.SignPublicKey(userPub, "test-key-id", "testuser")
	if err != nil {
		t.Fatalf("SignPublicKey() error = %v", err)
	}

	marshaled := MarshalCertificate(cert)
	if len(marshaled) == 0 {
		t.Error("MarshalCertificate() returned empty bytes")
	}

	// Verify it can be parsed back
	parsed, err := ParseCertificate(marshaled)
	if err != nil {
		t.Fatalf("ParseCertificate() error = %v", err)
	}

	if parsed.KeyId != cert.KeyId {
		t.Errorf("Parsed KeyId = %q, want %q", parsed.KeyId, cert.KeyId)
	}
}

func TestParsePublicKey(t *testing.T) {
	userPub, _ := generateTestUserKey(t)
	marshaled := ssh.MarshalAuthorizedKey(userPub)

	parsed, err := ParsePublicKey(marshaled)
	if err != nil {
		t.Fatalf("ParsePublicKey() error = %v", err)
	}

	if parsed.Type() != userPub.Type() {
		t.Errorf("Parsed key type = %q, want %q", parsed.Type(), userPub.Type())
	}
}

func TestParsePublicKeyInvalid(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"Empty input", []byte{}},
		{"Invalid format", []byte("not a valid key")},
		{"Partial key", []byte("ssh-ed25519 AAAA")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePublicKey(tt.input)
			if err == nil {
				t.Error("ParsePublicKey() expected error for invalid input")
			}
		})
	}
}

func TestParseCertificate(t *testing.T) {
	caKey := generateTestCAKey(t)
	ca, err := NewCA(caKey, 12, nil)
	if err != nil {
		t.Fatalf("Failed to create CA: %v", err)
	}

	userPub, _ := generateTestUserKey(t)
	cert, err := ca.SignPublicKey(userPub, "test-key-id", "testuser")
	if err != nil {
		t.Fatalf("SignPublicKey() error = %v", err)
	}

	marshaled := MarshalCertificate(cert)

	parsed, err := ParseCertificate(marshaled)
	if err != nil {
		t.Fatalf("ParseCertificate() error = %v", err)
	}

	if parsed.Serial != cert.Serial {
		t.Errorf("Parsed Serial = %d, want %d", parsed.Serial, cert.Serial)
	}
}

func TestParseCertificateInvalid(t *testing.T) {
	// Test with a regular public key (not a certificate)
	userPub, _ := generateTestUserKey(t)
	marshaled := ssh.MarshalAuthorizedKey(userPub)

	_, err := ParseCertificate(marshaled)
	if err == nil {
		t.Error("ParseCertificate() expected error for non-certificate")
	}
}

func TestGetCertInfo(t *testing.T) {
	caKey := generateTestCAKey(t)
	ca, err := NewCA(caKey, 12, nil)
	if err != nil {
		t.Fatalf("Failed to create CA: %v", err)
	}

	userPub, _ := generateTestUserKey(t)
	cert, err := ca.SignPublicKey(userPub, "test-key-id", "testuser")
	if err != nil {
		t.Fatalf("SignPublicKey() error = %v", err)
	}

	info := GetCertInfo(cert)

	if info.KeyID != "test-key-id" {
		t.Errorf("KeyID = %q, want %q", info.KeyID, "test-key-id")
	}

	if len(info.Principals) != 1 || info.Principals[0] != "testuser" {
		t.Errorf("Principals = %v, want [testuser]", info.Principals)
	}

	if info.IsExpired {
		t.Error("New certificate should not be expired")
	}

	if info.TimeLeft <= 0 {
		t.Error("TimeLeft should be positive for new certificate")
	}

	// TimeLeft should be approximately 12 hours
	expectedTimeLeft := 12 * time.Hour
	if info.TimeLeft < expectedTimeLeft-time.Minute || info.TimeLeft > expectedTimeLeft+time.Minute {
		t.Errorf("TimeLeft = %v, want approximately %v", info.TimeLeft, expectedTimeLeft)
	}
}

func TestGetCertInfoExpired(t *testing.T) {
	caKey := generateTestCAKey(t)
	// Create CA with 0 hour validity (already expired)
	ca, err := NewCA(caKey, 0, nil)
	if err != nil {
		t.Fatalf("Failed to create CA: %v", err)
	}

	userPub, _ := generateTestUserKey(t)
	cert, err := ca.SignPublicKey(userPub, "test-key-id", "testuser")
	if err != nil {
		t.Fatalf("SignPublicKey() error = %v", err)
	}

	info := GetCertInfo(cert)

	if !info.IsExpired {
		t.Error("Certificate with 0 validity should be expired")
	}
}

func TestSignPublicKeyForGitHub(t *testing.T) {
	caKey := generateTestCAKey(t)
	ca, err := NewCA(caKey, 12, nil)
	if err != nil {
		t.Fatalf("Failed to create CA: %v", err)
	}

	userPub, _ := generateTestUserKey(t)

	tests := []struct {
		name           string
		keyID          string
		githubUsername string
		githubHost     string
		wantLoginExt   string
	}{
		{
			name:           "GitHub.com (default)",
			keyID:          "test-github-key",
			githubUsername: "octocat",
			githubHost:     "",
			wantLoginExt:   "login@github.com",
		},
		{
			name:           "GitHub Enterprise",
			keyID:          "test-ghe-key",
			githubUsername: "enterprise-user",
			githubHost:     "github.company.com",
			wantLoginExt:   "login@github.company.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert, err := ca.SignPublicKeyForGitHub(userPub, tt.keyID, tt.githubUsername, tt.githubHost)
			if err != nil {
				t.Fatalf("SignPublicKeyForGitHub() error = %v", err)
			}

			// Verify certificate properties
			if cert.KeyId != tt.keyID {
				t.Errorf("KeyId = %q, want %q", cert.KeyId, tt.keyID)
			}

			if len(cert.ValidPrincipals) != 1 || cert.ValidPrincipals[0] != tt.githubUsername {
				t.Errorf("ValidPrincipals = %v, want [%s]", cert.ValidPrincipals, tt.githubUsername)
			}

			if cert.CertType != ssh.UserCert {
				t.Errorf("CertType = %d, want %d (UserCert)", cert.CertType, ssh.UserCert)
			}

			// Verify validity period
			now := time.Now()
			validAfter := time.Unix(int64(cert.ValidAfter), 0)
			validBefore := time.Unix(int64(cert.ValidBefore), 0)

			if validAfter.After(now) {
				t.Errorf("ValidAfter (%v) is in the future", validAfter)
			}

			expectedExpiry := now.Add(12 * time.Hour)
			if validBefore.Before(expectedExpiry.Add(-time.Minute)) || validBefore.After(expectedExpiry.Add(time.Minute)) {
				t.Errorf("ValidBefore = %v, want approximately %v", validBefore, expectedExpiry)
			}

			// Verify standard extensions
			expectedExtensions := []string{
				"permit-agent-forwarding",
				"permit-port-forwarding",
				"permit-pty",
				"permit-user-rc",
			}
			for _, ext := range expectedExtensions {
				if _, ok := cert.Extensions[ext]; !ok {
					t.Errorf("Missing extension: %s", ext)
				}
			}

			// Verify GitHub login extension
			loginValue, ok := cert.Extensions[tt.wantLoginExt]
			if !ok {
				t.Errorf("Missing GitHub login extension: %s", tt.wantLoginExt)
			}
			if loginValue != tt.githubUsername {
				t.Errorf("Login extension %s = %q, want %q", tt.wantLoginExt, loginValue, tt.githubUsername)
			}
		})
	}
}

func TestSignPublicKeyForGitHubWithPrincipals(t *testing.T) {
	caKey := generateTestCAKey(t)
	principals := []string{"git", "deploy", "admin"}
	ca, err := NewCA(caKey, 12, principals)
	if err != nil {
		t.Fatalf("Failed to create CA: %v", err)
	}

	userPub, _ := generateTestUserKey(t)

	cert, err := ca.SignPublicKeyForGitHub(userPub, "test-key-id", "octocat", "github.com")
	if err != nil {
		t.Fatalf("SignPublicKeyForGitHub() error = %v", err)
	}

	// With configured principals, should use those instead of username
	if len(cert.ValidPrincipals) != len(principals) {
		t.Errorf("ValidPrincipals length = %d, want %d", len(cert.ValidPrincipals), len(principals))
	}
	for i, p := range principals {
		if cert.ValidPrincipals[i] != p {
			t.Errorf("ValidPrincipals[%d] = %q, want %q", i, cert.ValidPrincipals[i], p)
		}
	}

	// Verify login extension still uses the username
	loginValue, ok := cert.Extensions["login@github.com"]
	if !ok {
		t.Error("Missing GitHub login extension: login@github.com")
	}
	if loginValue != "octocat" {
		t.Errorf("Login extension = %q, want %q", loginValue, "octocat")
	}
}

func TestSignPublicKeyForGitLab(t *testing.T) {
	caKey := generateTestCAKey(t)
	ca, err := NewCA(caKey, 12, nil)
	if err != nil {
		t.Fatalf("Failed to create CA: %v", err)
	}

	userPub, _ := generateTestUserKey(t)

	tests := []struct {
		name           string
		keyID          string
		gitlabUsername string
		gitlabHost     string
		wantLoginExt   string
	}{
		{
			name:           "GitLab.com (default)",
			keyID:          "test-gitlab-key",
			gitlabUsername: "gitlabuser",
			gitlabHost:     "",
			wantLoginExt:   "login@gitlab.com",
		},
		{
			name:           "GitLab Self-Managed",
			keyID:          "test-gitlab-sm-key",
			gitlabUsername: "enterprise-user",
			gitlabHost:     "gitlab.company.com",
			wantLoginExt:   "login@gitlab.company.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert, err := ca.SignPublicKeyForGitLab(userPub, tt.keyID, tt.gitlabUsername, tt.gitlabHost)
			if err != nil {
				t.Fatalf("SignPublicKeyForGitLab() error = %v", err)
			}

			// Verify certificate properties
			if cert.KeyId != tt.keyID {
				t.Errorf("KeyId = %q, want %q", cert.KeyId, tt.keyID)
			}

			if len(cert.ValidPrincipals) != 1 || cert.ValidPrincipals[0] != tt.gitlabUsername {
				t.Errorf("ValidPrincipals = %v, want [%s]", cert.ValidPrincipals, tt.gitlabUsername)
			}

			if cert.CertType != ssh.UserCert {
				t.Errorf("CertType = %d, want %d (UserCert)", cert.CertType, ssh.UserCert)
			}

			// Verify validity period
			now := time.Now()
			validAfter := time.Unix(int64(cert.ValidAfter), 0)
			validBefore := time.Unix(int64(cert.ValidBefore), 0)

			if validAfter.After(now) {
				t.Errorf("ValidAfter (%v) is in the future", validAfter)
			}

			expectedExpiry := now.Add(12 * time.Hour)
			if validBefore.Before(expectedExpiry.Add(-time.Minute)) || validBefore.After(expectedExpiry.Add(time.Minute)) {
				t.Errorf("ValidBefore = %v, want approximately %v", validBefore, expectedExpiry)
			}

			// Verify standard extensions
			expectedExtensions := []string{
				"permit-agent-forwarding",
				"permit-port-forwarding",
				"permit-pty",
				"permit-user-rc",
			}
			for _, ext := range expectedExtensions {
				if _, ok := cert.Extensions[ext]; !ok {
					t.Errorf("Missing extension: %s", ext)
				}
			}

			// Verify GitLab login extension
			loginValue, ok := cert.Extensions[tt.wantLoginExt]
			if !ok {
				t.Errorf("Missing GitLab login extension: %s", tt.wantLoginExt)
			}
			if loginValue != tt.gitlabUsername {
				t.Errorf("Login extension %s = %q, want %q", tt.wantLoginExt, loginValue, tt.gitlabUsername)
			}
		})
	}
}

func TestSignPublicKeyForGitLabWithPrincipals(t *testing.T) {
	caKey := generateTestCAKey(t)
	principals := []string{"git", "deploy", "admin"}
	ca, err := NewCA(caKey, 12, principals)
	if err != nil {
		t.Fatalf("Failed to create CA: %v", err)
	}

	userPub, _ := generateTestUserKey(t)

	cert, err := ca.SignPublicKeyForGitLab(userPub, "test-key-id", "gitlabuser", "gitlab.com")
	if err != nil {
		t.Fatalf("SignPublicKeyForGitLab() error = %v", err)
	}

	// With configured principals, should use those instead of username
	if len(cert.ValidPrincipals) != len(principals) {
		t.Errorf("ValidPrincipals length = %d, want %d", len(cert.ValidPrincipals), len(principals))
	}
	for i, p := range principals {
		if cert.ValidPrincipals[i] != p {
			t.Errorf("ValidPrincipals[%d] = %q, want %q", i, cert.ValidPrincipals[i], p)
		}
	}

	// Verify login extension still uses the username
	loginValue, ok := cert.Extensions["login@gitlab.com"]
	if !ok {
		t.Error("Missing GitLab login extension: login@gitlab.com")
	}
	if loginValue != "gitlabuser" {
		t.Errorf("Login extension = %q, want %q", loginValue, "gitlabuser")
	}
}
