//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

// Store the URL string to be retrieved by Go
static NSString* pendingURL = nil;

// URL handler delegate
@interface URLHandler : NSObject
+ (void)registerHandler;
@end

@implementation URLHandler

+ (void)registerHandler {
    // Register for GetURL Apple Events
    [[NSAppleEventManager sharedAppleEventManager]
        setEventHandler:self
        andSelector:@selector(handleGetURLEvent:withReplyEvent:)
        forEventClass:kInternetEventClass
        andEventID:kAEGetURL];
}

+ (void)handleGetURLEvent:(NSAppleEventDescriptor *)event withReplyEvent:(NSAppleEventDescriptor *)replyEvent {
    NSString *urlString = [[event paramDescriptorForKeyword:keyDirectObject] stringValue];
    if (urlString) {
        @synchronized(self) {
            pendingURL = [urlString copy];
        }
    }
}

@end

void registerURLHandler() {
    [URLHandler registerHandler];
}

// Check if there's a pending URL and return it (clears the pending URL)
const char* getPendingURL() {
    @synchronized([URLHandler class]) {
        if (pendingURL != nil) {
            const char* result = [pendingURL UTF8String];
            pendingURL = nil;
            return result;
        }
    }
    return NULL;
}
*/
import "C"
import (
	"encoding/base64"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/shawntz/cassh/internal/ca"
	"github.com/shawntz/cassh/internal/config"
)

// registerURLSchemeHandler registers the app to handle cassh:// URLs
func registerURLSchemeHandler() {
	C.registerURLHandler()
	log.Println("Registered URL scheme handler for cassh://")

	// Start polling for URLs in background
	go pollForURLs()
}

// pollForURLs checks for incoming URLs periodically
func pollForURLs() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		urlCStr := C.getPendingURL()
		if urlCStr != nil {
			urlString := C.GoString(urlCStr)
			go handleReceivedURL(urlString)
		}
	}
}

func handleReceivedURL(urlString string) {
	// Recover from any panic to prevent app crash
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in handleReceivedURL: %v", r)
			sendNotification("cassh Error", "Failed to process URL", false)
		}
	}()

	log.Printf("Received URL: %s", urlString)

	// Parse the URL
	u, err := url.Parse(urlString)
	if err != nil {
		log.Printf("Failed to parse URL: %v", err)
		return
	}

	// Handle different URL paths
	switch u.Host {
	case "install-cert":
		handleInstallCertURL(u)
	default:
		log.Printf("Unknown URL path: %s", u.Host)
	}
}

// handleInstallCertURL handles cassh://install-cert?cert=BASE64&connection_id=ID
func handleInstallCertURL(u *url.URL) {
	// Recover from any panic to prevent app crash
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in handleInstallCertURL: %v", r)
			sendNotification("cassh Error", "Failed to install certificate", false)
		}
	}()

	// Safety check - ensure config is loaded
	if cfg == nil {
		log.Println("Config not loaded, cannot install certificate")
		sendNotification("cassh Error", "App not fully initialized", false)
		return
	}

	query := u.Query()

	// Get certificate (base64 encoded)
	certB64 := query.Get("cert")
	if certB64 == "" {
		log.Println("No certificate in URL")
		sendNotification("cassh Error", "No certificate provided in URL", false)
		return
	}

	// Decode base64 - try RawURLEncoding first (no padding, URL-safe chars)
	// This matches the JavaScript: btoa().replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
	certBytes, err := base64.RawURLEncoding.DecodeString(certB64)
	if err != nil {
		// Try URL encoding with padding
		certBytes, err = base64.URLEncoding.DecodeString(certB64)
		if err != nil {
			// Try standard base64
			certBytes, err = base64.StdEncoding.DecodeString(certB64)
			if err != nil {
				log.Printf("Failed to decode certificate: %v", err)
				sendNotification("cassh Error", "Failed to decode certificate", false)
				return
			}
		}
	}

	cert := string(certBytes)

	// Validate cert
	if _, err := ca.ParseCertificate([]byte(cert)); err != nil {
		log.Printf("Invalid certificate: %v", err)
		sendNotification("cassh Error", "Invalid certificate received", false)
		return
	}

	// Get optional connection ID
	connectionID := query.Get("connection_id")

	// Find the connection to install cert for
	var conn *config.Connection
	if connectionID != "" {
		conn = cfg.User.GetConnection(connectionID)
	} else if len(cfg.User.Connections) > 0 {
		// Default to first enterprise connection
		for i := range cfg.User.Connections {
			if cfg.User.Connections[i].Type == config.ConnectionTypeEnterprise {
				conn = &cfg.User.Connections[i]
				break
			}
		}
	}

	// Determine paths
	var certPath, keyPath string
	var gheURL string
	if conn != nil {
		certPath = conn.SSHCertPath
		keyPath = conn.SSHKeyPath
		gheURL = "https://" + conn.GitHubHost
	} else {
		// Legacy fallback
		certPath = cfg.User.SSHCertPath
		keyPath = cfg.User.SSHKeyPath
		gheURL = cfg.Policy.GitHubEnterpriseURL
	}

	// Write cert
	if err := os.WriteFile(certPath, []byte(cert), 0644); err != nil {
		log.Printf("Failed to write cert: %v", err)
		sendNotification("cassh Error", "Failed to save certificate", false)
		return
	}

	// Add to ssh-agent
	if err := exec.Command("ssh-add", keyPath).Run(); err != nil {
		log.Printf("Warning: ssh-add failed: %v", err)
	}

	// Ensure SSH config is correct for this connection
	if conn != nil {
		if err := ensureSSHConfigForConnection(conn); err != nil {
			log.Printf("Warning: failed to configure SSH config: %v", err)
		}
	} else if gheURL != "" && !strings.HasPrefix(gheURL, "https://github.com") {
		// Legacy fallback
		if err := ensureSSHConfig(gheURL, keyPath); err != nil {
			log.Printf("Warning: failed to configure SSH config: %v", err)
		}
	}

	log.Println("Certificate installed successfully via URL scheme")

	// Parse cert to get expiration info
	parsedCert, _ := ca.ParseCertificate([]byte(cert))
	certInfo := ca.GetCertInfo(parsedCert)

	// Send success notification with time remaining
	connName := "GitHub Enterprise"
	if conn != nil {
		connName = conn.Name
	}
	timeRemaining := formatDuration(certInfo.TimeLeft)
	sendNotification("Certificate Activated",
		fmt.Sprintf("%s is now active. Valid for %s.", connName, timeRemaining),
		false)

	// Update connection status
	if conn != nil {
		for i, c := range cfg.User.Connections {
			if c.ID == conn.ID {
				go updateConnectionStatus(i)
				break
			}
		}
	}
}
