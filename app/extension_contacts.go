package app

import (
	extcontactsbe "github.com/caseylabs/mailven/extensions/contacts/backend"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// initContactsExtension wires the Contacts extension's Bridge into App
// during Startup. All bridge logic lives in extensions/contacts/backend/
// bridge.go; this file exists ONLY so the host can supply the bridge with
// its host-provided dependencies (settings store, paths, db, event emitter)
// and so the embedded-field promotion makes the bridge methods Wails-bindable.
//
// The bridge lazy-initializes its Contacts-specific state (stores, per-
// extension SQLite, API wrapper) inside `ensureInit` on the first enabled
// method call. When Contacts is disabled in settings, zero work happens
// beyond the ~80-byte Bridge struct allocation — this is how the
// lightweight-by-default promise is held.
func (a *App) initContactsExtension() {
	// Per-extension Core handle for cross-extension coreapi calls (source
	// management via ListSources / LinkAccountSource). Distinct from the
	// Core constructed in the Startup Register loop but functionally
	// equivalent — both point at the same app, scoped to the same
	// extension identity for Auth routing.
	contactsCore := newCoreForExtension(a, a.contactsExt)

	// Single emitter for all contacts:* frontend events (conflict, changed).
	emit := func(eventName string, payload any) {
		wailsRuntime.EventsEmit(a.ctx, eventName, payload)
	}

	a.ContactsBridge = extcontactsbe.NewContactsBridge(extcontactsbe.ContactsBridgeDeps{
		SettingsStore: a.settingsStore,
		DataDir:       a.paths.Data,
		Emitter:       emit,
		// All shared contact data and authentication flow through the SDK.
		Core: contactsCore,
	})
	a.contactsExt.AttachBridge(a.ContactsBridge)

	// Live-refresh the contact list after any source sync (background
	// scheduler, post-add, or manual). The core carddav syncer fires a generic
	// completion callback; this host-wiring file translates it into the
	// extension's `contacts:changed` frontend event, reusing the same Emitter
	// as contacts:conflict. (carddavSyncer is constructed earlier in Startup.)
	if a.carddavSyncer != nil {
		a.carddavSyncer.SetSyncErrorHandler(a.reportDAVCertificateError)
		a.carddavSyncer.SetSyncCompleteHandler(func(sourceID string) {
			emit("contacts:changed", map[string]string{"sourceId": sourceID})
		})
	}

	// All OAuth slot resolution lives in internal/oauth2/core_provider.go
	// now — google-contacts and microsoft-contacts are owned there. The
	// contacts extension package carries no OAuth client vars of its own.
}
