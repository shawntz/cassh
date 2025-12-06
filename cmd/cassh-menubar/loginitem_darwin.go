//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework ServiceManagement -framework AppKit

#import <Foundation/Foundation.h>
#import <ServiceManagement/ServiceManagement.h>
#import <AppKit/AppKit.h>

// Check if running on macOS 13+ (Ventura)
int supportsLoginItems() {
    if (@available(macOS 13.0, *)) {
        return 1;
    }
    return 0;
}

// Register as a login item (macOS 13+)
// Returns: 0 = success, 1 = already enabled, -1 = error
int registerLoginItem() {
    if (@available(macOS 13.0, *)) {
        SMAppService *service = [SMAppService mainAppService];

        // Check current status
        if (service.status == SMAppServiceStatusEnabled) {
            NSLog(@"cassh: Already registered as login item");
            return 1;
        }

        NSError *error = nil;
        BOOL success = [service registerAndReturnError:&error];

        if (success) {
            NSLog(@"cassh: Registered as login item");
            return 0;
        } else {
            NSLog(@"cassh: Failed to register login item: %@", error.localizedDescription);
            return -1;
        }
    }
    return -1;
}

// Unregister as a login item (macOS 13+)
int unregisterLoginItem() {
    if (@available(macOS 13.0, *)) {
        SMAppService *service = [SMAppService mainAppService];

        NSError *error = nil;
        BOOL success = [service unregisterAndReturnError:&error];

        if (success) {
            NSLog(@"cassh: Unregistered as login item");
            return 0;
        } else {
            NSLog(@"cassh: Failed to unregister login item: %@", error.localizedDescription);
            return -1;
        }
    }
    return -1;
}

// Get login item status
// Returns: 0 = not registered, 1 = enabled, 2 = requires approval, -1 = error/unsupported
int getLoginItemStatus() {
    if (@available(macOS 13.0, *)) {
        SMAppService *service = [SMAppService mainAppService];

        switch (service.status) {
            case SMAppServiceStatusNotRegistered:
                return 0;
            case SMAppServiceStatusEnabled:
                return 1;
            case SMAppServiceStatusRequiresApproval:
                return 2;
            case SMAppServiceStatusNotFound:
                return -1;
            default:
                return -1;
        }
    }
    return -1;
}

// Open System Preferences to Login Items
void openLoginItemsPreferences() {
    if (@available(macOS 13.0, *)) {
        NSURL *url = [NSURL URLWithString:@"x-apple.systempreferences:com.apple.LoginItems-Settings.extension"];
        [[NSWorkspace sharedWorkspace] openURL:url];
    } else {
        // Older macOS - open Users & Groups
        NSURL *url = [NSURL URLWithString:@"x-apple.systempreferences:com.apple.preferences.users"];
        [[NSWorkspace sharedWorkspace] openURL:url];
    }
}
*/
import "C"
import "log"

// LoginItemStatus represents the current login item registration status
type LoginItemStatus int

const (
	LoginItemNotRegistered    LoginItemStatus = 0
	LoginItemEnabled          LoginItemStatus = 1
	LoginItemRequiresApproval LoginItemStatus = 2
	LoginItemError            LoginItemStatus = -1
)

// supportsModernLoginItems returns true if running on macOS 13+ with SMAppService support
func supportsModernLoginItems() bool {
	return C.supportsLoginItems() == 1
}

// registerAsLoginItem registers the app as a login item using SMAppService
// This will trigger the system notification on first registration
func registerAsLoginItem() error {
	if !supportsModernLoginItems() {
		log.Println("Login items require macOS 13+, falling back to LaunchAgent")
		installLaunchAgent()
		return nil
	}

	result := C.registerLoginItem()
	switch result {
	case 0:
		log.Println("Successfully registered as login item")
	case 1:
		log.Println("Already registered as login item")
	default:
		log.Println("Failed to register as login item, falling back to LaunchAgent")
		installLaunchAgent()
	}
	return nil
}

// unregisterAsLoginItem removes the app from login items
func unregisterAsLoginItem() {
	if supportsModernLoginItems() {
		C.unregisterLoginItem()
	}
}

// getLoginItemStatus returns the current login item status
func getLoginItemStatus() LoginItemStatus {
	return LoginItemStatus(C.getLoginItemStatus())
}

// openLoginItemsSettings opens System Preferences to the Login Items section
func openLoginItemsSettings() {
	C.openLoginItemsPreferences()
}
