//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -Wno-deprecated-declarations
#cgo LDFLAGS: -framework Foundation -framework UserNotifications -framework AppKit

#import <Foundation/Foundation.h>
#import <UserNotifications/UserNotifications.h>
#import <AppKit/AppKit.h>

// Global flag to indicate a notification was clicked (polled by Go)
static volatile int notificationAction = 0; // 0=none, 1=renew, 2=open

int getAndClearNotificationAction() {
    int action = notificationAction;
    notificationAction = 0;
    return action;
}

// Notification delegate to handle click actions
@interface CasshNotificationDelegate : NSObject <UNUserNotificationCenterDelegate>
@end

@implementation CasshNotificationDelegate

// Called when user interacts with a notification while app is in foreground
- (void)userNotificationCenter:(UNUserNotificationCenter *)center
       willPresentNotification:(UNNotification *)notification
         withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completionHandler {
    // Show the notification even when app is in foreground
    completionHandler(UNNotificationPresentationOptionBanner | UNNotificationPresentationOptionSound);
}

// Called when user clicks on a notification or action button
- (void)userNotificationCenter:(UNUserNotificationCenter *)center
didReceiveNotificationResponse:(UNNotificationResponse *)response
         withCompletionHandler:(void (^)(void))completionHandler {

    NSString *actionID = response.actionIdentifier;
    NSString *categoryID = response.notification.request.content.categoryIdentifier;

    // Handle different actions by setting the flag
    if ([actionID isEqualToString:UNNotificationDefaultActionIdentifier]) {
        // User clicked the notification itself
        if ([categoryID isEqualToString:@"CERT_EXPIRED"] ||
            [categoryID isEqualToString:@"CERT_EXPIRING"]) {
            notificationAction = 1; // renew
        } else {
            notificationAction = 2; // open
        }
    } else if ([actionID isEqualToString:@"RENEW_ACTION"]) {
        notificationAction = 1; // renew
    }
    // DISMISS_ACTION: notificationAction stays 0

    // Bring app to front
    [[NSApplication sharedApplication] activateIgnoringOtherApps:YES];

    completionHandler();
}

@end

static CasshNotificationDelegate *notificationDelegate = nil;

static void setupNotificationDelegate() {
    if (notificationDelegate == nil) {
        notificationDelegate = [[CasshNotificationDelegate alloc] init];
    }
    UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
    center.delegate = notificationDelegate;

    // Define notification categories with actions
    UNNotificationAction *renewAction = [UNNotificationAction actionWithIdentifier:@"RENEW_ACTION"
                                                                             title:@"Renew Now"
                                                                           options:UNNotificationActionOptionForeground];
    UNNotificationAction *dismissAction = [UNNotificationAction actionWithIdentifier:@"DISMISS_ACTION"
                                                                               title:@"Dismiss"
                                                                             options:UNNotificationActionOptionNone];

    // Category for certificate expiring/expired
    UNNotificationCategory *certExpiringCategory = [UNNotificationCategory categoryWithIdentifier:@"CERT_EXPIRING"
                                                                                          actions:@[renewAction, dismissAction]
                                                                                intentIdentifiers:@[]
                                                                                          options:UNNotificationCategoryOptionNone];
    UNNotificationCategory *certExpiredCategory = [UNNotificationCategory categoryWithIdentifier:@"CERT_EXPIRED"
                                                                                         actions:@[renewAction, dismissAction]
                                                                               intentIdentifiers:@[]
                                                                                         options:UNNotificationCategoryOptionNone];

    // General notification category
    UNNotificationCategory *generalCategory = [UNNotificationCategory categoryWithIdentifier:@"GENERAL"
                                                                                     actions:@[]
                                                                           intentIdentifiers:@[]
                                                                                     options:UNNotificationCategoryOptionNone];

    [center setNotificationCategories:[NSSet setWithObjects:certExpiringCategory, certExpiredCategory, generalCategory, nil]];
}

// Check if running inside an app bundle (required for UNUserNotificationCenter)
int isRunningInAppBundle() {
    NSBundle *mainBundle = [NSBundle mainBundle];
    NSString *bundlePath = mainBundle.bundlePath;
    return [bundlePath hasSuffix:@".app"] ? 1 : 0;
}

void requestNotificationPermission() {
    // UNUserNotificationCenter requires an app bundle - skip if running as standalone binary
    if (!isRunningInAppBundle()) {
        NSLog(@"cassh: Not running in app bundle, notifications disabled");
        return;
    }

    UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
    [center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound | UNAuthorizationOptionBadge)
                          completionHandler:^(BOOL granted, NSError * _Nullable error) {
        if (error) {
            NSLog(@"Notification permission error: %@", error);
        } else if (granted) {
            dispatch_async(dispatch_get_main_queue(), ^{
                setupNotificationDelegate();
            });
        }
    }];
}

void sendNativeNotificationWithCategory(const char *title, const char *body, const char *category) {
    NSLog(@"sendNativeNotificationWithCategory called: %s - %s", title, body);

    // Check if running in app bundle
    if (!isRunningInAppBundle()) {
        NSLog(@"cassh: Not in app bundle, skipping notification");
        return;
    }

    NSString *nsTitle = [NSString stringWithUTF8String:title];
    NSString *nsBody = [NSString stringWithUTF8String:body];
    NSString *nsCategory = category ? [NSString stringWithUTF8String:category] : @"GENERAL";

    UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];

    UNMutableNotificationContent *content = [[UNMutableNotificationContent alloc] init];
    content.title = nsTitle;
    content.body = nsBody;
    content.sound = [UNNotificationSound defaultSound];
    content.categoryIdentifier = nsCategory;

    NSString *identifier = [[NSUUID UUID] UUIDString];
    UNNotificationRequest *request = [UNNotificationRequest requestWithIdentifier:identifier
                                                                          content:content
                                                                          trigger:nil];

    [center addNotificationRequest:request withCompletionHandler:^(NSError * _Nullable error) {
        if (error) {
            NSLog(@"Failed to send notification: %@", error);
        }
    }];
}

void sendNativeNotification(const char *title, const char *body) {
    sendNativeNotificationWithCategory(title, body, "GENERAL");
}
*/
import "C"
import (
	"log"
	"time"
	"unsafe"
)

func initNotifications() {
	C.requestNotificationPermission()

	// Start a goroutine to poll for notification actions
	go pollNotificationActions()
}

// pollNotificationActions checks for notification click actions periodically
func pollNotificationActions() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		action := int(C.getAndClearNotificationAction())
		if action == 0 {
			continue
		}

		log.Printf("Notification action: %d", action)
		switch action {
		case 1: // renew
			// Open the first enterprise connection for renewal
			for _, conn := range cfg.User.Connections {
				if conn.Type == "enterprise" {
					handleConnectionAction(conn.ID)
					break
				}
			}
		case 2: // open
			openSetupWizard()
		}
	}
}

func sendNativeNotification(title, body string) {
	log.Printf("sendNativeNotification: title=%s, body=%s", title, body)
	cTitle := C.CString(title)
	cBody := C.CString(body)
	defer C.free(unsafe.Pointer(cTitle))
	defer C.free(unsafe.Pointer(cBody))

	C.sendNativeNotification(cTitle, cBody)
	log.Println("sendNativeNotification: C function called")
}

// sendNotificationWithCategory sends a notification with a specific category for actions
func sendNotificationWithCategory(title, body, category string) {
	cTitle := C.CString(title)
	cBody := C.CString(body)
	cCategory := C.CString(category)
	defer C.free(unsafe.Pointer(cTitle))
	defer C.free(unsafe.Pointer(cBody))
	defer C.free(unsafe.Pointer(cCategory))

	C.sendNativeNotificationWithCategory(cTitle, cBody, cCategory)
}
