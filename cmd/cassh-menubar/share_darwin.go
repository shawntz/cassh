//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

// Custom button subclass to handle click animation
@interface ShareCopyButton : NSButton
@property (nonatomic, strong) NSTextField *statusLabel;
@property (nonatomic, strong) NSString *shareText;
@property (nonatomic) BOOL isCopied;
@end

@implementation ShareCopyButton

- (void)performCopy:(id)sender {
    if (self.isCopied) return;

    // Copy to clipboard
    NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
    [pasteboard clearContents];
    [pasteboard setString:self.shareText forType:NSPasteboardTypeString];

    // Animate to success state
    self.isCopied = YES;

    // Update button appearance
    [NSAnimationContext runAnimationGroup:^(NSAnimationContext *context) {
        context.duration = 0.2;
        context.allowsImplicitAnimation = YES;

        // Change button title with checkmark
        [self setTitle:@"✓ Copied to clipboard"];

        // Update button color to green
        self.bezelColor = [NSColor colorWithRed:0.133 green:0.773 blue:0.369 alpha:1.0];
    }];

    // Update status label
    if (self.statusLabel) {
        self.statusLabel.stringValue = @"Ready to paste!";
        self.statusLabel.textColor = [NSColor colorWithRed:0.133 green:0.773 blue:0.369 alpha:1.0];
    }

    // Reset after delay
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(3.0 * NSEC_PER_SEC)), dispatch_get_main_queue(), ^{
        [NSAnimationContext runAnimationGroup:^(NSAnimationContext *context) {
            context.duration = 0.2;
            context.allowsImplicitAnimation = YES;
            [self setTitle:@"Copy to Clipboard"];
            self.bezelColor = [NSColor controlAccentColor];
        }];

        if (self.statusLabel) {
            self.statusLabel.stringValue = @"Click to copy";
            self.statusLabel.textColor = [NSColor tertiaryLabelColor];
        }

        self.isCopied = NO;
    });
}

@end

void showShareDialog() {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSString *shareText = @"Check out cassh - SSH Key & Certificate Manager for GitHub! Ephemeral SSH certificates for enterprise, automatic key rotation for personal accounts. https://github.com/shawntz/cassh";

        // Create the share window
        NSWindow *shareWindow = [[NSWindow alloc] initWithContentRect:NSMakeRect(0, 0, 440, 280)
                                                            styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskClosable
                                                              backing:NSBackingStoreBuffered
                                                                defer:NO];
        [shareWindow setTitle:@"Share cassh"];
        [shareWindow center];

        // Float above other windows (same level as setup wizard)
        [shareWindow setLevel:NSFloatingWindowLevel];

        // Set dark appearance
        if (@available(macOS 10.14, *)) {
            shareWindow.appearance = [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua];
        }

        NSView *contentView = [shareWindow contentView];
        contentView.wantsLayer = YES;
        contentView.layer.backgroundColor = [[NSColor colorWithWhite:0.1 alpha:1.0] CGColor];

        // Header icon and title
        NSImageView *iconView = [[NSImageView alloc] initWithFrame:NSMakeRect(20, 220, 40, 40)];
        NSImage *appIcon = [NSApp applicationIconImage];
        if (appIcon) {
            [iconView setImage:appIcon];
        }
        [iconView setImageScaling:NSImageScaleProportionallyUpOrDown];
        [contentView addSubview:iconView];

        NSTextField *titleLabel = [[NSTextField alloc] initWithFrame:NSMakeRect(70, 228, 350, 28)];
        [titleLabel setStringValue:@"Send your friends some cassh..."];
        [titleLabel setFont:[NSFont boldSystemFontOfSize:18]];
        [titleLabel setTextColor:[NSColor whiteColor]];
        [titleLabel setBezeled:NO];
        [titleLabel setDrawsBackground:NO];
        [titleLabel setEditable:NO];
        [titleLabel setSelectable:NO];
        [contentView addSubview:titleLabel];

        // Subtitle
        NSTextField *subtitleLabel = [[NSTextField alloc] initWithFrame:NSMakeRect(20, 185, 400, 20)];
        [subtitleLabel setStringValue:@"Copy the message below to share with colleagues:"];
        [subtitleLabel setFont:[NSFont systemFontOfSize:12]];
        [subtitleLabel setTextColor:[NSColor secondaryLabelColor]];
        [subtitleLabel setBezeled:NO];
        [subtitleLabel setDrawsBackground:NO];
        [subtitleLabel setEditable:NO];
        [subtitleLabel setSelectable:NO];
        [contentView addSubview:subtitleLabel];

        // Share text box with border
        NSBox *textBox = [[NSBox alloc] initWithFrame:NSMakeRect(20, 85, 400, 90)];
        [textBox setBoxType:NSBoxCustom];
        [textBox setBorderColor:[NSColor colorWithWhite:0.25 alpha:1.0]];
        [textBox setFillColor:[NSColor colorWithWhite:0.05 alpha:1.0]];
        [textBox setBorderWidth:1.0];
        [textBox setCornerRadius:8.0];
        [textBox setTitlePosition:NSNoTitle];
        [contentView addSubview:textBox];

        // Share text (inside the box)
        NSTextField *shareTextField = [[NSTextField alloc] initWithFrame:NSMakeRect(12, 8, 376, 70)];
        [shareTextField setStringValue:shareText];
        [shareTextField setFont:[NSFont systemFontOfSize:12]];
        [shareTextField setTextColor:[NSColor colorWithWhite:0.7 alpha:1.0]];
        [shareTextField setBezeled:NO];
        [shareTextField setDrawsBackground:NO];
        [shareTextField setEditable:NO];
        [shareTextField setSelectable:YES];
        [shareTextField setLineBreakMode:NSLineBreakByWordWrapping];
        [[shareTextField cell] setWraps:YES];
        [[textBox contentView] addSubview:shareTextField];

        // Status label
        NSTextField *statusLabel = [[NSTextField alloc] initWithFrame:NSMakeRect(20, 55, 200, 20)];
        [statusLabel setStringValue:@"Click to copy"];
        [statusLabel setFont:[NSFont systemFontOfSize:11]];
        [statusLabel setTextColor:[NSColor tertiaryLabelColor]];
        [statusLabel setBezeled:NO];
        [statusLabel setDrawsBackground:NO];
        [statusLabel setEditable:NO];
        [statusLabel setSelectable:NO];
        [contentView addSubview:statusLabel];

        // Copy button
        ShareCopyButton *copyBtn = [[ShareCopyButton alloc] initWithFrame:NSMakeRect(20, 15, 400, 36)];
        [copyBtn setTitle:@"Copy to Clipboard"];
        [copyBtn setBezelStyle:NSBezelStyleRounded];
        if (@available(macOS 10.14, *)) {
            copyBtn.bezelColor = [NSColor controlAccentColor];
        }
        copyBtn.wantsLayer = YES;
        [copyBtn setFont:[NSFont systemFontOfSize:14 weight:NSFontWeightMedium]];
        copyBtn.shareText = shareText;
        copyBtn.statusLabel = statusLabel;
        copyBtn.isCopied = NO;
        [copyBtn setTarget:copyBtn];
        [copyBtn setAction:@selector(performCopy:)];
        [contentView addSubview:copyBtn];

        [shareWindow makeKeyAndOrderFront:nil];
        [NSApp activateIgnoringOtherApps:YES];
    });
}
*/
import "C"

func showShareDialog() {
	C.showShareDialog()
}
