#import <UserNotifications/UserNotifications.h>

// Forward declaration of the Go callback (defined via //export in notifier_darwin.go)
extern void goNotificationCallback(char *accountId, char *folderId, char *threadId);

// Delegate that handles notification interactions and foreground presentation
@interface MailvenNotificationDelegate : NSObject <UNUserNotificationCenterDelegate>
@end

@implementation MailvenNotificationDelegate

- (void)userNotificationCenter:(UNUserNotificationCenter *)center
didReceiveNotificationResponse:(UNNotificationResponse *)response
         withCompletionHandler:(void (^)(void))completionHandler {
    NSDictionary *userInfo = response.notification.request.content.userInfo;
    NSString *accountId = userInfo[@"accountId"] ?: @"";
    NSString *folderId  = userInfo[@"folderId"]  ?: @"";
    NSString *threadId  = userInfo[@"threadId"]  ?: @"";

    goNotificationCallback(
        (char *)[accountId UTF8String],
        (char *)[folderId  UTF8String],
        (char *)[threadId  UTF8String]
    );

    completionHandler();
}

- (void)userNotificationCenter:(UNUserNotificationCenter *)center
       willPresentNotification:(UNNotification *)notification
         withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completionHandler {
    // Show notification and play sound even when app is in foreground.
    // UNNotificationPresentationOptionBanner requires macOS 11.0+.
    UNNotificationPresentationOptions opts = UNNotificationPresentationOptionSound;
    if (@available(macOS 11.0, *)) {
        opts |= UNNotificationPresentationOptionBanner;
    }
    completionHandler(opts);
}

@end

static MailvenNotificationDelegate *notifDelegate = nil;

// setupNotifications initializes UNUserNotificationCenter and requests authorization.
// Dispatches to the main queue since UNUserNotificationCenter delegate must be
// configured on the main thread for reliable callback delivery.
void setupNotifications(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
        notifDelegate = [[MailvenNotificationDelegate alloc] init];
        center.delegate = notifDelegate;

        [center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound)
                              completionHandler:^(BOOL granted, NSError *error) {
            if (error != nil) {
                NSLog(@"[Mailven] Notification authorization error: %@", error);
                return;
            }
            NSLog(@"[Mailven] Notification authorization granted: %d", granted);
        }];
    });
}

// showNotification creates and submits a notification asynchronously — never blocks.
void showNotification(const char *title, const char *body,
                      const char *accountId, const char *folderId, const char *threadId) {
    // Autorelease pool for the Go goroutine thread (which has no pool of its own).
    // The NSStrings are autoreleased by stringWithUTF8String:, and dispatch_async
    // retains them when copying the block to the heap, so they survive past the pool drain.
    @autoreleasepool {
    NSString *nsTitle     = [NSString stringWithUTF8String:title];
    NSString *nsBody      = [NSString stringWithUTF8String:body];
    NSString *nsAccountId = [NSString stringWithUTF8String:accountId];
    NSString *nsFolderId  = [NSString stringWithUTF8String:folderId];
    NSString *nsThreadId  = [NSString stringWithUTF8String:threadId];

    dispatch_async(dispatch_get_main_queue(), ^{
        UNMutableNotificationContent *content = [[UNMutableNotificationContent alloc] init];
        content.title = nsTitle;
        content.body  = nsBody;
        content.sound = [UNNotificationSound defaultSound];
        content.userInfo = @{
            @"accountId": nsAccountId,
            @"folderId":  nsFolderId,
            @"threadId":  nsThreadId,
        };

        NSString *identifier = [[NSUUID UUID] UUIDString];
        UNNotificationRequest *request = [UNNotificationRequest requestWithIdentifier:identifier
                                                                              content:content
                                                                              trigger:nil];

        [[UNUserNotificationCenter currentNotificationCenter]
            addNotificationRequest:request
             withCompletionHandler:^(NSError *error) {
                if (error != nil) {
                    NSLog(@"[Mailven] Failed to deliver notification: %@", error);
                }
            }];
    });
    } // @autoreleasepool
}

// cancelNotifications removes all delivered notifications.
void cancelNotifications(void) {
    [[UNUserNotificationCenter currentNotificationCenter] removeAllDeliveredNotifications];
}
