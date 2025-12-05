// cassh-cli is the headless CLI for CI/CD and server environments
// It generates certificates without a GUI, using device code flow or service principals
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/shawntz/cassh/internal/ca"
	"github.com/shawntz/cassh/internal/config"
)

var (
	serverURL  string
	keyPath    string
	certPath   string
	outputJSON bool
	showStatus bool
	autoAdd    bool
)

func init() {
	flag.StringVar(&serverURL, "server", "", "cassh server URL (or set CASSH_SERVER)")
	flag.StringVar(&keyPath, "key", "", "SSH private key path")
	flag.StringVar(&certPath, "cert", "", "SSH certificate output path")
	flag.BoolVar(&outputJSON, "json", false, "Output in JSON format")
	flag.BoolVar(&showStatus, "status", false, "Show current certificate status")
	flag.BoolVar(&autoAdd, "add", true, "Automatically add key to ssh-agent")
}

func main() {
	flag.Parse()
	log.SetFlags(0)

	// Load config
	userCfg, _ := config.LoadUserConfig()
	if userCfg == nil {
		defaults := config.DefaultUserConfig()
		userCfg = &defaults
	}

	// Apply defaults from config
	if keyPath == "" {
		keyPath = userCfg.SSHKeyPath
	}
	if certPath == "" {
		certPath = userCfg.SSHCertPath
	}
	if serverURL == "" {
		serverURL = os.Getenv("CASSH_SERVER")
		if serverURL == "" {
			// Try to load from policy
			policy, err := config.LoadPolicy(config.PolicyPath())
			if err == nil {
				serverURL = policy.ServerBaseURL
			}
		}
	}

	if showStatus {
		displayStatus()
		return
	}

	// Validate required params
	if serverURL == "" {
		fatal("Server URL required. Use --server or set CASSH_SERVER")
	}

	if err := generateCert(); err != nil {
		fatal("Failed to generate certificate: %v", err)
	}
}

func displayStatus() {
	certData, err := os.ReadFile(certPath)
	if err != nil {
		if outputJSON {
			outputResult(map[string]interface{}{
				"valid": false,
				"error": "No certificate found",
			})
		} else {
			fmt.Println("❌ No certificate found")
			fmt.Printf("   Expected at: %s\n", certPath)
		}
		os.Exit(1)
	}

	cert, err := ca.ParseCertificate(certData)
	if err != nil {
		if outputJSON {
			outputResult(map[string]interface{}{
				"valid": false,
				"error": "Invalid certificate",
			})
		} else {
			fmt.Println("❌ Invalid certificate")
		}
		os.Exit(1)
	}

	info := ca.GetCertInfo(cert)

	if outputJSON {
		outputResult(map[string]interface{}{
			"valid":        !info.IsExpired,
			"expires_at":   info.ValidBefore,
			"time_left":    info.TimeLeft.String(),
			"key_id":       info.KeyID,
			"principals":   info.Principals,
			"serial":       info.Serial,
			"valid_after":  info.ValidAfter,
			"valid_before": info.ValidBefore,
		})
	} else {
		if info.IsExpired {
			fmt.Println("❌ Certificate EXPIRED")
		} else {
			fmt.Println("✅ Certificate valid")
		}
		fmt.Printf("   Key ID:     %s\n", info.KeyID)
		fmt.Printf("   Principals: %v\n", info.Principals)
		fmt.Printf("   Valid:      %s - %s\n",
			info.ValidAfter.Format(time.RFC3339),
			info.ValidBefore.Format(time.RFC3339))
		if !info.IsExpired {
			fmt.Printf("   Time left:  %s\n", formatDuration(info.TimeLeft))
		}
	}

	if info.IsExpired {
		os.Exit(1)
	}
}

func generateCert() error {
	// Ensure SSH key exists
	if err := ensureSSHKey(keyPath); err != nil {
		return fmt.Errorf("key generation failed: %w", err)
	}

	// Read public key
	pubKeyPath := keyPath + ".pub"
	pubKeyData, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read public key: %w", err)
	}

	if !outputJSON {
		fmt.Println("🔐 Starting certificate generation...")
		fmt.Printf("   Server: %s\n", serverURL)
		fmt.Printf("   Key:    %s\n", keyPath)
	}

	// For CLI mode, we use device code flow
	// Open browser for auth (interactive mode)
	authURL := fmt.Sprintf("%s/auth/start?pubkey=%s",
		serverURL,
		url.QueryEscape(string(pubKeyData)),
	)

	if !outputJSON {
		fmt.Println("\n📱 Opening browser for authentication...")
		fmt.Println("   If browser doesn't open, visit:")
		fmt.Printf("   %s\n", authURL)
	}

	// Try to open browser
	openBrowser(authURL)

	// Poll local loopback for certificate (if menubar is running)
	// or wait for manual paste
	if !outputJSON {
		fmt.Println("\n⏳ Waiting for certificate...")
		fmt.Println("   Complete authentication in browser, then either:")
		fmt.Println("   1. Click 'Auto-Install' button (if cassh.app is running)")
		fmt.Println("   2. Copy certificate and paste below, then press Enter twice:")
	}

	// Try polling loopback first
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cert, err := pollForCert(ctx)
	if err != nil {
		// Fall back to manual input
		if !outputJSON {
			cert, err = readCertFromStdin()
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("certificate not received: %w", err)
		}
	}

	// Write certificate
	if err := os.WriteFile(certPath, []byte(cert), 0644); err != nil {
		return fmt.Errorf("failed to write certificate: %w", err)
	}

	// Add to ssh-agent
	if autoAdd {
		if err := exec.Command("ssh-add", keyPath).Run(); err != nil {
			if !outputJSON {
				fmt.Printf("⚠️  Warning: ssh-add failed: %v\n", err)
			}
		}
	}

	// Verify certificate
	certData, _ := os.ReadFile(certPath)
	parsedCert, err := ca.ParseCertificate(certData)
	if err != nil {
		return fmt.Errorf("certificate verification failed: %w", err)
	}

	info := ca.GetCertInfo(parsedCert)

	if outputJSON {
		outputResult(map[string]interface{}{
			"success":    true,
			"cert_path":  certPath,
			"key_path":   keyPath,
			"expires_at": info.ValidBefore,
			"time_left":  info.TimeLeft.String(),
		})
	} else {
		fmt.Println("\n✅ Certificate installed successfully!")
		fmt.Printf("   Expires: %s\n", info.ValidBefore.Format(time.RFC3339))
		fmt.Printf("   Time left: %s\n", formatDuration(info.TimeLeft))
	}

	return nil
}

func pollForCert(ctx context.Context) (string, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	loopbackURL := "http://127.0.0.1:52849/status"

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			resp, err := http.Get(loopbackURL)
			if err != nil {
				continue // Loopback not available
			}
			defer resp.Body.Close()

			var status struct {
				Valid bool `json:"valid"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
				continue
			}

			if status.Valid {
				// Cert was installed via loopback, read it
				certData, err := os.ReadFile(certPath)
				if err == nil {
					return string(certData), nil
				}
			}
		}
	}
}

func readCertFromStdin() (string, error) {
	var cert string
	var emptyLines int

	for {
		var line string
		_, err := fmt.Scanln(&line)
		if err == io.EOF || line == "" {
			emptyLines++
			if emptyLines >= 2 {
				break
			}
			continue
		}
		emptyLines = 0
		cert += line + "\n"
	}

	if cert == "" {
		return "", fmt.Errorf("no certificate provided")
	}

	// Validate it's a cert
	if _, err := ca.ParseCertificate([]byte(cert)); err != nil {
		return "", fmt.Errorf("invalid certificate: %w", err)
	}

	return cert, nil
}

func ensureSSHKey(keyPath string) error {
	if _, err := os.Stat(keyPath); err == nil {
		return nil
	}

	// Ensure directory
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return err
	}

	// Use ssh-keygen for reliable key generation
	cmd := exec.Command("ssh-keygen",
		"-t", "ed25519",
		"-f", keyPath,
		"-N", "",
		"-C", "cassh-generated",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch os := os.Getenv("GOOS"); os {
	case "darwin", "":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	}
	if cmd != nil {
		cmd.Start()
	}
}

func outputResult(data interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}

func fatal(format string, args ...interface{}) {
	if outputJSON {
		outputResult(map[string]interface{}{
			"error": fmt.Sprintf(format, args...),
		})
	} else {
		fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	}
	os.Exit(1)
}
