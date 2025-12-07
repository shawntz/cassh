//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

void showAboutDialog(const char *version, const char *buildCommit) {
    // Copy strings BEFORE dispatch_async to avoid race condition with Go's defer
    NSString *nsVersion = (version && strlen(version) > 0) ? [NSString stringWithUTF8String:version] : @"dev";
    NSString *nsBuildCommit = (buildCommit && strlen(buildCommit) > 0) ? [NSString stringWithUTF8String:buildCommit] : @"dev";

    dispatch_async(dispatch_get_main_queue(), ^{

        // Create the about window
        NSWindow *aboutWindow = [[NSWindow alloc] initWithContentRect:NSMakeRect(0, 0, 360, 320)
                                                           styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskClosable
                                                             backing:NSBackingStoreBuffered
                                                               defer:NO];
        [aboutWindow setTitle:@"About cassh"];
        [aboutWindow center];

        // Float above other windows (same level as setup wizard)
        [aboutWindow setLevel:NSFloatingWindowLevel];

        NSView *contentView = [aboutWindow contentView];

        // App Icon
        NSImageView *iconView = [[NSImageView alloc] initWithFrame:NSMakeRect(140, 220, 80, 80)];
        NSImage *appIcon = [NSApp applicationIconImage];
        if (appIcon) {
            [iconView setImage:appIcon];
        }
        [iconView setImageScaling:NSImageScaleProportionallyUpOrDown];
        [contentView addSubview:iconView];

        // App Name
        NSTextField *nameLabel = [[NSTextField alloc] initWithFrame:NSMakeRect(20, 185, 320, 30)];
        [nameLabel setStringValue:@"cassh"];
        [nameLabel setFont:[NSFont boldSystemFontOfSize:20]];
        [nameLabel setAlignment:NSTextAlignmentCenter];
        [nameLabel setBezeled:NO];
        [nameLabel setDrawsBackground:NO];
        [nameLabel setEditable:NO];
        [nameLabel setSelectable:NO];
        [contentView addSubview:nameLabel];

        // Tagline
        NSTextField *taglineLabel = [[NSTextField alloc] initWithFrame:NSMakeRect(20, 165, 320, 20)];
        [taglineLabel setStringValue:@"SSH Key & Certificate Manager for GitHub"];
        [taglineLabel setFont:[NSFont systemFontOfSize:12]];
        [taglineLabel setTextColor:[NSColor secondaryLabelColor]];
        [taglineLabel setAlignment:NSTextAlignmentCenter];
        [taglineLabel setBezeled:NO];
        [taglineLabel setDrawsBackground:NO];
        [taglineLabel setEditable:NO];
        [taglineLabel setSelectable:NO];
        [contentView addSubview:taglineLabel];

        // Version
        NSTextField *versionLabel = [[NSTextField alloc] initWithFrame:NSMakeRect(20, 140, 320, 20)];
        NSString *versionText = [NSString stringWithFormat:@"%@", nsVersion];
        [versionLabel setStringValue:versionText];
        [versionLabel setFont:[NSFont systemFontOfSize:11]];
        [versionLabel setTextColor:[NSColor tertiaryLabelColor]];
        [versionLabel setAlignment:NSTextAlignmentCenter];
        [versionLabel setBezeled:NO];
        [versionLabel setDrawsBackground:NO];
        [versionLabel setEditable:NO];
        [versionLabel setSelectable:YES];
        [contentView addSubview:versionLabel];

        // Build/Commit
        NSTextField *buildLabel = [[NSTextField alloc] initWithFrame:NSMakeRect(20, 120, 320, 20)];
        NSString *buildText = [NSString stringWithFormat:@"(%@)", nsBuildCommit];
        [buildLabel setStringValue:buildText];
        [buildLabel setFont:[NSFont monospacedSystemFontOfSize:10 weight:NSFontWeightRegular]];
        [buildLabel setTextColor:[NSColor tertiaryLabelColor]];
        [buildLabel setAlignment:NSTextAlignmentCenter];
        [buildLabel setBezeled:NO];
        [buildLabel setDrawsBackground:NO];
        [buildLabel setEditable:NO];
        [buildLabel setSelectable:YES];
        [contentView addSubview:buildLabel];

        // GitHub Button
        NSButton *githubBtn = [[NSButton alloc] initWithFrame:NSMakeRect(30, 70, 90, 32)];
        [githubBtn setTitle:@"GitHub"];
        [githubBtn setBezelStyle:NSBezelStyleRounded];
        [githubBtn setTarget:nil];
        [githubBtn setAction:@selector(openGitHub:)];
        [contentView addSubview:githubBtn];

        // Docs Button
        NSButton *docsBtn = [[NSButton alloc] initWithFrame:NSMakeRect(135, 70, 90, 32)];
        [docsBtn setTitle:@"Docs"];
        [docsBtn setBezelStyle:NSBezelStyleRounded];
        [docsBtn setTarget:nil];
        [docsBtn setAction:@selector(openDocs:)];
        [contentView addSubview:docsBtn];

        // Sponsor Button
        NSButton *sponsorBtn = [[NSButton alloc] initWithFrame:NSMakeRect(240, 70, 90, 32)];
        [sponsorBtn setTitle:@"Sponsor ❤️"];
        [sponsorBtn setBezelStyle:NSBezelStyleRounded];
        [sponsorBtn setTarget:nil];
        [sponsorBtn setAction:@selector(openSponsor:)];
        [contentView addSubview:sponsorBtn];

        // Copyright
        NSTextField *copyrightLabel = [[NSTextField alloc] initWithFrame:NSMakeRect(20, 25, 320, 30)];
        [copyrightLabel setStringValue:@"© 2025 Shawn Schwartz\nOpen source under Apache 2.0"];
        [copyrightLabel setFont:[NSFont systemFontOfSize:10]];
        [copyrightLabel setTextColor:[NSColor tertiaryLabelColor]];
        [copyrightLabel setAlignment:NSTextAlignmentCenter];
        [copyrightLabel setBezeled:NO];
        [copyrightLabel setDrawsBackground:NO];
        [copyrightLabel setEditable:NO];
        [copyrightLabel setSelectable:NO];
        [contentView addSubview:copyrightLabel];

        [aboutWindow makeKeyAndOrderFront:nil];
        [NSApp activateIgnoringOtherApps:YES];
    });
}

// Button action handlers - these need to be implemented as a category or delegate
@interface NSApplication (AboutActions)
- (void)openGitHub:(id)sender;
- (void)openDocs:(id)sender;
- (void)openSponsor:(id)sender;
@end

@implementation NSApplication (AboutActions)
- (void)openGitHub:(id)sender {
    [[NSWorkspace sharedWorkspace] openURL:[NSURL URLWithString:@"https://github.com/shawntz/cassh"]];
}
- (void)openDocs:(id)sender {
    [[NSWorkspace sharedWorkspace] openURL:[NSURL URLWithString:@"https://shawnschwartz.com/cassh"]];
}
- (void)openSponsor:(id)sender {
    [[NSWorkspace sharedWorkspace] openURL:[NSURL URLWithString:@"https://github.com/sponsors/shawntz"]];
}
@end
*/
import "C"
import (
	"os/exec"
	"strings"
	"unsafe"
)

func showAbout() {
	cVersion := C.CString(version)
	cBuildCommit := C.CString(buildCommit)
	defer C.free(unsafe.Pointer(cVersion))
	defer C.free(unsafe.Pointer(cBuildCommit))

	C.showAboutDialog(cVersion, cBuildCommit)
}

// showUninstallConfirmation shows a native confirmation dialog for uninstall
// Returns true if user confirms, false otherwise
func showUninstallConfirmation() bool {
	script := `
		set response to display dialog "Are you sure you want to uninstall cassh?\n\nThis will remove:\n• All SSH keys created by cassh\n• Configuration files\n• LaunchAgent (auto-start)\n• The cassh application" with title "Uninstall cassh" buttons {"Cancel", "Uninstall"} default button "Cancel" cancel button "Cancel" with icon caution
		return button returned of response
	`
	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return false // User cancelled or error
	}
	return strings.TrimSpace(string(output)) == "Uninstall"
}
