//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework WebKit

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

// Store references to prevent garbage collection
static NSWindow* setupWindow = nil;
static WKWebView* webView = nil;

// Forward declaration for Go callback - defined via //export in Go code
// Note: CGO generates declaration without 'const', so we match that
void handleCasshURL(char* url);

// Navigation delegate to handle page loads and custom URL schemes
@interface WebViewDelegate : NSObject <WKNavigationDelegate>
@end

@implementation WebViewDelegate
- (void)webView:(WKWebView *)webView didFinishNavigation:(WKNavigation *)navigation {
    // Page finished loading
}

- (void)webView:(WKWebView *)webView didFailNavigation:(WKNavigation *)navigation withError:(NSError *)error {
    NSLog(@"WebView navigation failed: %@", error.localizedDescription);
}

// Intercept navigation requests to handle cassh:// URLs
- (void)webView:(WKWebView *)webView decidePolicyForNavigationAction:(WKNavigationAction *)navigationAction decisionHandler:(void (^)(WKNavigationActionPolicy))decisionHandler {
    NSURL *url = navigationAction.request.URL;

    // Handle cassh:// URL scheme
    if ([[url scheme] isEqualToString:@"cassh"]) {
        NSLog(@"WebView intercepted cassh:// URL: %@", url.absoluteString);
        // Call Go handler (cast away const since Go doesn't use const)
        handleCasshURL((char*)[url.absoluteString UTF8String]);
        decisionHandler(WKNavigationActionPolicyCancel);
        return;
    }

    // Allow all other URLs
    decisionHandler(WKNavigationActionPolicyAllow);
}
@end

// Window delegate to handle close without quitting app
@interface WindowDelegate : NSObject <NSWindowDelegate>
@end

@implementation WindowDelegate
- (BOOL)windowShouldClose:(NSWindow *)sender {
    // Allow window to close
    return YES;
}

- (void)windowWillClose:(NSNotification *)notification {
    // Clean up references but don't quit the app
    setupWindow = nil;
    webView = nil;
}
@end

static WebViewDelegate* navDelegate = nil;
static WindowDelegate* windowDelegate = nil;

void openWebViewWindow(const char* urlStr, const char* title, int width, int height) {
    dispatch_async(dispatch_get_main_queue(), ^{
        // If window already exists, just bring it to front
        if (setupWindow != nil && [setupWindow isVisible]) {
            [setupWindow makeKeyAndOrderFront:nil];
            [NSApp activateIgnoringOtherApps:YES];

            // Navigate to the URL (might be different page)
            NSURL *url = [NSURL URLWithString:[NSString stringWithUTF8String:urlStr]];
            NSURLRequest *request = [NSURLRequest requestWithURL:url];
            [webView loadRequest:request];
            return;
        }

        // Create window
        NSRect frame = NSMakeRect(0, 0, width, height);
        NSWindowStyleMask style = NSWindowStyleMaskTitled |
                                  NSWindowStyleMaskClosable |
                                  NSWindowStyleMaskMiniaturizable |
                                  NSWindowStyleMaskResizable;

        setupWindow = [[NSWindow alloc] initWithContentRect:frame
                                                  styleMask:style
                                                    backing:NSBackingStoreBuffered
                                                      defer:NO];

        [setupWindow setTitle:[NSString stringWithUTF8String:title]];
        [setupWindow setMinSize:NSMakeSize(500, 400)];

        // Set window delegate to prevent app quit on close
        if (windowDelegate == nil) {
            windowDelegate = [[WindowDelegate alloc] init];
        }
        [setupWindow setDelegate:windowDelegate];

        // Prevent window from releasing on close (menu bar app behavior)
        [setupWindow setReleasedWhenClosed:NO];

        // Center on screen
        [setupWindow center];

        // Create WebView configuration
        WKWebViewConfiguration *config = [[WKWebViewConfiguration alloc] init];

        // Enable developer extras for debugging (optional)
        [config.preferences setValue:@YES forKey:@"developerExtrasEnabled"];

        // Create WebView
        webView = [[WKWebView alloc] initWithFrame:frame configuration:config];

        // Set navigation delegate
        if (navDelegate == nil) {
            navDelegate = [[WebViewDelegate alloc] init];
        }
        webView.navigationDelegate = navDelegate;

        // Set as content view
        [setupWindow setContentView:webView];

        // Load URL
        NSURL *url = [NSURL URLWithString:[NSString stringWithUTF8String:urlStr]];
        NSURLRequest *request = [NSURLRequest requestWithURL:url];
        [webView loadRequest:request];

        // Show window
        [setupWindow makeKeyAndOrderFront:nil];
        [NSApp activateIgnoringOtherApps:YES];
    });
}

void closeWebViewWindow() {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (setupWindow != nil) {
            [setupWindow close];
            setupWindow = nil;
            webView = nil;
        }
    });
}

int isWebViewWindowVisible() {
    __block int visible = 0;
    dispatch_sync(dispatch_get_main_queue(), ^{
        if (setupWindow != nil && [setupWindow isVisible]) {
            visible = 1;
        }
    });
    return visible;
}
*/
import "C"
import "fmt"

// openNativeWebView opens a native WebKit window with the given URL
func openNativeWebView(url, title string, width, height int) {
	C.openWebViewWindow(C.CString(url), C.CString(title), C.int(width), C.int(height))
}

// closeNativeWebView closes the WebView window if open
func closeNativeWebView() {
	C.closeWebViewWindow()
}

// isNativeWebViewOpen returns true if the WebView window is visible
func isNativeWebViewOpen() bool {
	return C.isWebViewWindowVisible() == 1
}

// openSetupWindow opens the setup wizard in a native window
func openSetupWindow() {
	url := fmt.Sprintf("http://localhost:%d/setup", loopbackPort)
	openNativeWebView(url, "cassh Setup", 800, 800)
}

// openSettingsWindow opens settings in a native window
func openSettingsWindow() {
	url := fmt.Sprintf("http://localhost:%d/setup", loopbackPort)
	openNativeWebView(url, "cassh Settings", 800, 800)
}
