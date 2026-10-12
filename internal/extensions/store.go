package extensions

import storekit "github.com/caseylabs/mailven/internal/kit/extensionstore"

// Shared extension store compatibility surface. New consumers use the kit.
type Migration = storekit.Migration
type Store = storekit.Store
var OpenStore = storekit.OpenStore
