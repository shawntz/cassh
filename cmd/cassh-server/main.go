// cassh-server is the certificate authority web service
// Serves the meme landing page and handles OIDC authentication with Entra ID
package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/shawntz/cassh/internal/ca"
	"github.com/shawntz/cassh/internal/config"
	"github.com/shawntz/cassh/internal/memes"
	"github.com/shawntz/cassh/internal/oidc"
)

//go:embed templates/*
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

// Holds the cassh server state
type Server struct {
	config  *config.ServerConfig
	auth    *oidc.Authenticator
	ca      *ca.CertificateAuthority
	tmpl    *template.Template
	devMode bool
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting cassh-server...")

	// Load server config (file + env var overrides)
	policyPath := os.Getenv("CASSH_POLICY_PATH")
	if policyPath == "" {
		// Try default locations
		if _, err := os.Stat("cassh.policy.toml"); err == nil {
			policyPath = "cassh.policy.toml"
		}
	}

	cfg, err := config.LoadServerConfig(policyPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	devMode := cfg.IsDevMode()
	if devMode {
		log.Println("⚠️  DEVELOPMENT MODE - authentication is mocked!")
	} else {
		// Validate required config in production mode
		if err := cfg.Validate(); err != nil {
			log.Fatalf("Configuration error: %v", err)
		}
	}

	// Initialize CA (skip in dev mode if no key)
	var certAuthority *ca.CertificateAuthority
	if cfg.CAPrivateKey != "" {
		certAuthority, err = ca.NewCA([]byte(cfg.CAPrivateKey), cfg.CertValidityHours, nil)
		if err != nil {
			log.Fatalf("Failed to initialize CA: %v", err)
		}
	} else if !devMode {
		log.Fatalf("CA private key is required in production mode")
	}

	// Initialize OIDC authenticator (only if not in devel mode)
	var auth *oidc.Authenticator
	if !devMode {
		ctx := context.Background()
		redirectURL := cfg.OIDCRedirectURL
		if redirectURL == "" {
			redirectURL = cfg.ServerBaseURL + "/auth/callback"
		}
		auth, err = oidc.NewAuthenticator(ctx, &oidc.EntraConfig{
			TenantID:     cfg.OIDCTenant,
			ClientID:     cfg.OIDCClientID,
			ClientSecret: cfg.OIDCClientSecret,
			RedirectURL:  redirectURL,
		})
		if err != nil {
			log.Fatalf("Failed to initialize OIDC: %v", err)
		}
	}

	// Parse templates
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	server := &Server{
		config:  cfg,
		auth:    auth,
		ca:      certAuthority,
		tmpl:    tmpl,
		devMode: devMode,
	}

	// Setup routes
	mux := http.NewServeMux()

	// Static files
	staticContent, _ := fs.Sub(staticFS, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticContent))))

	// Routes
	mux.HandleFunc("/", server.handleLanding)
	mux.HandleFunc("/auth/start", server.handleAuthStart)
	mux.HandleFunc("/auth/callback", server.handleAuthCallback)
	mux.HandleFunc("/auth/dev", server.handleDevAuth) // Dev mode mock auth
	mux.HandleFunc("/cert/issue", server.handleCertIssue)
	mux.HandleFunc("/health", server.handleHealth)

	// Start server
	addr := os.Getenv("CASSH_LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      logMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("Shutdown error: %v", err)
		}
	}()

	log.Printf("cassh-server listening on %s", addr)
	if devMode {
		log.Printf("🌐 Open http://localhost%s in your browser", addr)
	}
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

// handleLanding serves the meme-blessed landing page
func (s *Server) handleLanding(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Get pubkey from query params (sent by menubar app)
	pubKey := r.URL.Query().Get("pubkey")

	// Get random meme data
	memeData := memes.GetMemeData("random")

	data := struct {
		Meme       memes.MemeData
		PubKey     string
		ServerName string
		DevMode    bool
	}{
		Meme:       memeData,
		PubKey:     pubKey,
		ServerName: s.config.ServerBaseURL,
		DevMode:    s.devMode,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, "landing.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
	}
}

// handleAuthStart initiates the OIDC flow
func (s *Server) handleAuthStart(w http.ResponseWriter, r *http.Request) {
	pubKey := r.URL.Query().Get("pubkey")
	if pubKey == "" {
		http.Error(w, "Missing pubkey parameter", http.StatusBadRequest)
		return
	}

	// In devel mode, redirect to mock auth
	if s.devMode {
		http.Redirect(w, r, "/auth/dev?pubkey="+pubKey, http.StatusFound)
		return
	}

	authURL, err := s.auth.StartAuth(pubKey)
	if err != nil {
		log.Printf("Auth start error: %v", err)
		http.Error(w, "Failed to start authentication", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

// handleDevAuth is the mock auth for devel mode
func (s *Server) handleDevAuth(w http.ResponseWriter, r *http.Request) {
	if !s.devMode {
		http.Error(w, "Dev auth not available in production", http.StatusForbidden)
		return
	}

	pubKey := r.URL.Query().Get("pubkey")
	if pubKey == "" {
		http.Error(w, "Missing pubkey parameter", http.StatusBadRequest)
		return
	}

	// Mock user info
	userInfo := &oidc.UserInfo{
		Subject:       "dev-user-123",
		Email:         "developer@localhost",
		EmailVerified: true,
		Name:          "Local Developer",
		Username:      "devuser",
	}

	// Extract principal from OIDC claims based on config
	principal := extractPrincipal(userInfo, s.config.GitHubPrincipalSource)
	log.Printf("🔓 DEV AUTH: Mock user authenticated: %s (principal: %s)", userInfo.Email, principal)

	// Parse the user's public key
	sshPubKey, err := ca.ParsePublicKey([]byte(pubKey))
	if err != nil {
		log.Printf("Invalid public key: %v", err)
		http.Error(w, "Invalid public key format", http.StatusBadRequest)
		return
	}

	// Generate cert with GitHub login extension
	// Extract GitHub hostname from URL (e.g., "https://github.mycompany.com" -> "github.mycompany.com")
	githubHost := config.ExtractHostFromURL(s.config.GitHubEnterpriseURL)
	keyID := fmt.Sprintf("cassh:dev:%s:%d", userInfo.Email, time.Now().Unix())
	cert, err := s.ca.SignPublicKeyForGitHub(sshPubKey, keyID, principal, githubHost)
	if err != nil {
		log.Printf("Cert signing error: %v", err)
		http.Error(w, "Failed to generate certificate", http.StatusInternalServerError)
		return
	}

	log.Printf("🔓 DEV AUTH: Signed cert for principal=%s, login@%s=%s", principal, githubHost, principal)

	certData := ca.MarshalCertificate(cert)
	certInfo := ca.GetCertInfo(cert)

	// Render success page with cert
	memeData := memes.GetMemeData("random")
	data := struct {
		Meme     memes.MemeData
		Cert     string
		CertInfo *ca.CertInfo
		User     *oidc.UserInfo
		DevMode  bool
	}{
		Meme:     memeData,
		Cert:     string(certData),
		CertInfo: certInfo,
		User:     userInfo,
		DevMode:  true,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, "success.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
	}
}

// handleAuthCallback processes the OIDC callback from Entra ID
func (s *Server) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	if s.devMode {
		http.Error(w, "Use /auth/dev in development mode", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	userInfo, pubKey, err := s.auth.HandleCallback(ctx, r)
	if err != nil {
		log.Printf("Auth callback error: %v", err)
		http.Error(w, "Authentication failed: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// Extract principal from OIDC claims based on config
	principal := extractPrincipal(userInfo, s.config.GitHubPrincipalSource)
	log.Printf("User authenticated: %s (principal: %s)", userInfo.Email, principal)

	// Parse the user's public key
	sshPubKey, err := ca.ParsePublicKey([]byte(pubKey))
	if err != nil {
		log.Printf("Invalid public key: %v", err)
		http.Error(w, "Invalid public key format", http.StatusBadRequest)
		return
	}

	// Generate cert with GitHub login extension
	// Extract GitHub hostname from URL (e.g., "https://github.mycompany.com" -> "github.mycompany.com")
	githubHost := config.ExtractHostFromURL(s.config.GitHubEnterpriseURL)
	keyID := fmt.Sprintf("cassh:%s:%d", userInfo.Email, time.Now().Unix())
	cert, err := s.ca.SignPublicKeyForGitHub(sshPubKey, keyID, principal, githubHost)
	if err != nil {
		log.Printf("Cert signing error: %v", err)
		http.Error(w, "Failed to generate certificate", http.StatusInternalServerError)
		return
	}

	log.Printf("Signed cert for %s: principal=%s, login@%s=%s", userInfo.Email, principal, githubHost, principal)

	certData := ca.MarshalCertificate(cert)
	certInfo := ca.GetCertInfo(cert)

	// Render success page with cert
	memeData := memes.GetMemeData("random")
	data := struct {
		Meme     memes.MemeData
		Cert     string
		CertInfo *ca.CertInfo
		User     *oidc.UserInfo
		DevMode  bool
	}{
		Meme:     memeData,
		Cert:     string(certData),
		CertInfo: certInfo,
		User:     userInfo,
		DevMode:  false,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, "success.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
	}
}

// handleCertIssue is the API endpoint for menubar loopback listener
// This allows the browser to POST the cert directly to the local app
func (s *Server) handleCertIssue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// This endpoint returns JSON for the local menubar app
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		PubKey string `json:"pubkey"`
		Token  string `json:"token"` // OIDC token from authenticated session
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	// In production, verify the token here
	// For now, this endpoint requires the auth flow to have completed

	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"note":   "Use /auth/start flow for certificate generation",
	})
}

// handleHealth is the health check endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"dev_mode":  s.devMode,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// extractPrincipal derives the SSH certificate principal from OIDC user info based on config
// principalSource options:
//   - "email_prefix" (default): extract username before @ from email/UPN (e.g., "shawn@schwartz.so" -> "shawn")
//   - "email": use full email as principal
//   - "username": use the preferred_username claim as-is
func extractPrincipal(userInfo *oidc.UserInfo, principalSource string) string {
	switch principalSource {
	case "email":
		return userInfo.Email
	case "username":
		return userInfo.Username
	case "email_prefix", "":
		// Default: extract username part from email/UPN
		emailOrUPN := userInfo.Username
		if emailOrUPN == "" {
			emailOrUPN = userInfo.Email
		}
		if idx := strings.Index(emailOrUPN, "@"); idx != -1 {
			return emailOrUPN[:idx]
		}
		return emailOrUPN
	default:
		// Fallback to email_prefix behavior
		emailOrUPN := userInfo.Username
		if idx := strings.Index(emailOrUPN, "@"); idx != -1 {
			return emailOrUPN[:idx]
		}
		return emailOrUPN
	}
}
