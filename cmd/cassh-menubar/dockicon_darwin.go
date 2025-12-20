//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

// Forward declaration for Go callback - defined via //export in Go code
void handleDockIconClick();

// Application delegate to handle Dock icon clicks
@interface DockIconDelegate : NSObject <NSApplicationDelegate>
@end

@implementation DockIconDelegate
// Called when user clicks Dock icon
- (BOOL)applicationShouldHandleReopen:(NSApplication *)sender hasVisibleWindows:(BOOL)flag {
    // Call Go handler to open settings/setup window
    handleDockIconClick();
    return YES;
}
@end

static DockIconDelegate* dockDelegate = nil;

// Setup dock icon click handler
void setupDockIconHandler() {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (dockDelegate == nil) {
            dockDelegate = [[DockIconDelegate alloc] init];
            [NSApp setDelegate:dockDelegate];
        }
    });
}
*/
import "C"
import (
	"log"
)

// setupDockIconClickHandler initializes the Dock icon click handler
func setupDockIconClickHandler() {
	C.setupDockIconHandler()
}

//export handleDockIconClick
func handleDockIconClick() {
	log.Println("Dock icon clicked - opening settings window")
	openSetupWizard()
}
