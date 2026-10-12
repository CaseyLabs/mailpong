//go:build darwin

package notification

/*
#cgo CFLAGS: -x objective-c -mmacosx-version-min=10.14
#cgo LDFLAGS: -framework Foundation -framework UserNotifications

#include <stdlib.h>

// Implemented in notifier_darwin.m
void setupNotifications(void);
void showNotification(const char *title, const char *body,
                      const char *accountId, const char *folderId, const char *threadId);
void cancelNotifications(void);
*/
import "C"

import (
	"context"
	"sync"
	"unsafe"

	"github.com/caseylabs/mailven/internal/logging"
	"github.com/rs/zerolog"
)

// package-level singleton so the C callback can reach the Go instance
var darwinNotif *darwinNotifier

//export goNotificationCallback
func goNotificationCallback(accountId *C.char, folderId *C.char, threadId *C.char) {
	n := darwinNotif
	if n == nil {
		return
	}

	n.mu.RLock()
	handler := n.clickHandler
	n.mu.RUnlock()

	if handler == nil {
		return
	}

	data := NotificationData{
		AccountID: C.GoString(accountId),
		FolderID:  C.GoString(folderId),
		ThreadID:  C.GoString(threadId),
	}

	n.log.Info().
		Str("accountId", data.AccountID).
		Str("folderId", data.FolderID).
		Str("threadId", data.ThreadID).
		Msg("Notification clicked, invoking handler")

	// Dispatch to a goroutine — this callback runs on the macOS main thread,
	// and the handler calls Wails functions that dispatch_sync to the main
	// thread, which would deadlock if called directly.
	go handler(data)
}

// darwinNotifier uses UNUserNotificationCenter for notifications on macOS
// with click handling via inline CGo Objective-C.
type darwinNotifier struct {
	appName      string
	clickHandler ClickHandler
	mu           sync.RWMutex
	log          zerolog.Logger
}

func newPlatformNotifier(appName string, useDirectDBus bool) Notifier {
	// useDirectDBus is Linux-only, ignored on macOS
	return &darwinNotifier{
		appName: appName,
		log:     logging.WithComponent("notification"),
	}
}

func (n *darwinNotifier) Start(ctx context.Context) error {
	darwinNotif = n
	C.setupNotifications()
	n.log.Info().Msg("macOS notification support started (UNUserNotificationCenter)")
	return nil
}

func (n *darwinNotifier) Stop() {
	C.cancelNotifications()
	darwinNotif = nil
	n.log.Info().Msg("macOS notification listener stopped")
}

func (n *darwinNotifier) Show(notif Notification) (uint32, error) {
	cTitle := C.CString(notif.Title)
	defer C.free(unsafe.Pointer(cTitle))
	cBody := C.CString(notif.Body)
	defer C.free(unsafe.Pointer(cBody))
	cAccountID := C.CString(notif.Data.AccountID)
	defer C.free(unsafe.Pointer(cAccountID))
	cFolderID := C.CString(notif.Data.FolderID)
	defer C.free(unsafe.Pointer(cFolderID))
	cThreadID := C.CString(notif.Data.ThreadID)
	defer C.free(unsafe.Pointer(cThreadID))

	C.showNotification(cTitle, cBody, cAccountID, cFolderID, cThreadID)

	n.log.Debug().Str("title", notif.Title).Msg("Notification shown via UNUserNotificationCenter")
	return 0, nil
}

func (n *darwinNotifier) SetClickHandler(handler ClickHandler) {
	n.mu.Lock()
	n.clickHandler = handler
	n.mu.Unlock()
}
