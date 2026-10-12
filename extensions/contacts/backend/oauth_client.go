package backend

import (
	"fmt"
	coreapi "github.com/caseylabs/mailven/internal/core/api/v1"
	"net/http"
)

func (a *API) httpClientForSource(source *coreapi.ContactDAVSource, scope coreapi.AuthScope) (*http.Client, error) {
	if source == nil || a.data == nil {
		return nil, fmt.Errorf("contacts: source authentication unavailable")
	}
	return a.data.SourceHTTPClient(source.ID, []coreapi.AuthScope{scope})
}
