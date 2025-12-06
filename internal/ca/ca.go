// Handles SSH cert generation and signing
// Certs issued with configurable validity (default 12 hours)
package ca

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"
)

// CertificateAuthority manages SSH cert signing
type CertificateAuthority struct {
	signer        ssh.Signer
	validityHours int
	principals    []string
}

// NewCA creates a new certificate authority from a private key
func NewCA(privateKeyPEM []byte, validityHours int, principals []string) (*CertificateAuthority, error) {
	signer, err := ssh.ParsePrivateKey(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA private key: %w", err)
	}

	return &CertificateAuthority{
		signer:        signer,
		validityHours: validityHours,
		principals:    principals,
	}, nil
}

// SignPublicKey signs a user's public key, creating an SSH cert
// Deprecated: Use SignPublicKeyForGitHub instead for GitHub Enterprise
func (ca *CertificateAuthority) SignPublicKey(userPubKey ssh.PublicKey, keyID string, username string) (*ssh.Certificate, error) {
	return ca.SignPublicKeyForGitHub(userPubKey, keyID, username, "")
}

// SignPublicKeyForGitHub signs a user's public key with GitHub-specific extensions
// The githubHost should be the GHE hostname (e.g., "github.yourcompany.com") or empty for github.com
// The githubUsername is the user's GitHub/GHE username for the login extension
func (ca *CertificateAuthority) SignPublicKeyForGitHub(userPubKey ssh.PublicKey, keyID string, githubUsername string, githubHost string) (*ssh.Certificate, error) {
	// Generate random serial
	serialBytes := make([]byte, 8)
	if _, err := rand.Read(serialBytes); err != nil {
		return nil, fmt.Errorf("failed to generate serial: %w", err)
	}
	serial := binary.BigEndian.Uint64(serialBytes)

	now := time.Now()
	validAfter := uint64(now.Unix())
	validBefore := uint64(now.Add(time.Duration(ca.validityHours) * time.Hour).Unix())

	// Determine principals
	principals := ca.principals
	if len(principals) == 0 {
		principals = []string{githubUsername}
	}

	// Build extensions - these are REQUIRED for GitHub Enterprise
	// See: https://docs.github.com/en/enterprise-cloud@latest/organizations/managing-git-access-to-your-organizations-repositories/about-ssh-certificate-authorities
	extensions := map[string]string{
		"permit-agent-forwarding": "",
		"permit-port-forwarding":  "",
		"permit-pty":              "",
		"permit-user-rc":          "",
	}

	// Add the GitHub login extension - this is CRITICAL for GHE
	// Format: login@HOSTNAME=USERNAME
	// For github.com: login@github.com=username
	// For GHE: login@github.yourcompany.com=username
	if githubHost != "" {
		extensions[fmt.Sprintf("login@%s", githubHost)] = githubUsername
	} else {
		// Default to github.com
		extensions["login@github.com"] = githubUsername
	}

	cert := &ssh.Certificate{
		Key:             userPubKey,
		Serial:          serial,
		CertType:        ssh.UserCert,
		KeyId:           keyID,
		ValidPrincipals: principals,
		ValidAfter:      validAfter,
		ValidBefore:     validBefore,
		Permissions: ssh.Permissions{
			Extensions: extensions,
		},
	}

	if err := cert.SignCert(rand.Reader, ca.signer); err != nil {
		return nil, fmt.Errorf("failed to sign certificate: %w", err)
	}

	return cert, nil
}

// GenerateKeyPair creates a new Ed25519 keypair for the user
func GenerateKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate keypair: %w", err)
	}
	return pub, priv, nil
}

// MarshalCertificate converts a cert to the authorized_keys format
func MarshalCertificate(cert *ssh.Certificate) []byte {
	return ssh.MarshalAuthorizedKey(cert)
}

// MarshalPrivateKey converts an Ed25519 private key to PEM format
func MarshalPrivateKey(priv ed25519.PrivateKey) ([]byte, error) {
	// Use the ssh package to marshal to OpenSSH format
	block, err := ssh.MarshalPrivateKey(priv, "cassh generated key")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}
	return block.Bytes, nil
}

// ParsePublicKey parses an SSH public key from authorized_keys format
func ParsePublicKey(pubKeyBytes []byte) (ssh.PublicKey, error) {
	pub, _, _, _, err := ssh.ParseAuthorizedKey(pubKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}
	return pub, nil
}

// CertInfo contains human-readable cert information
type CertInfo struct {
	Serial      uint64
	KeyID       string
	Principals  []string
	ValidAfter  time.Time
	ValidBefore time.Time
	IsExpired   bool
	TimeLeft    time.Duration
}

// GetCertInfo extracts info from a cert for display
func GetCertInfo(cert *ssh.Certificate) *CertInfo {
	now := time.Now()
	validBefore := time.Unix(int64(cert.ValidBefore), 0)
	validAfter := time.Unix(int64(cert.ValidAfter), 0)

	return &CertInfo{
		Serial:      cert.Serial,
		KeyID:       cert.KeyId,
		Principals:  cert.ValidPrincipals,
		ValidAfter:  validAfter,
		ValidBefore: validBefore,
		IsExpired:   now.After(validBefore),
		TimeLeft:    validBefore.Sub(now),
	}
}

// ParseCertificate parses an SSH cert from file
func ParseCertificate(certBytes []byte) (*ssh.Certificate, error) {
	pub, _, _, _, err := ssh.ParseAuthorizedKey(certBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	cert, ok := pub.(*ssh.Certificate)
	if !ok {
		return nil, fmt.Errorf("not a certificate")
	}

	return cert, nil
}
