package app

import (
	"fmt"
	"net/http"
	"time"

	"github.com/caseylabs/mailven/internal/carddav"
	"github.com/caseylabs/mailven/internal/contactdata"
	coreapi "github.com/caseylabs/mailven/internal/core/api/v1"
	"github.com/caseylabs/mailven/internal/kit/davutil"
)

func (c contactsCoreImpl) Data() coreapi.ContactData {
	if c.app == nil || c.core == nil || c.core.extensionID != "contacts" {
		return nil
	}
	return contactdata.New(c.app.contactStore, c.app.carddavStore, c.app.db.DB,
		c.sourceHTTPClient, c.davClient)
}

func (c contactsCoreImpl) sourceHTTPClient(id string, scopes []coreapi.AuthScope) (*http.Client, error) {
	source, err := c.app.carddavStore.GetSource(id)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, fmt.Errorf("contact source %s not found", id)
	}
	if source.AccountID != nil && *source.AccountID != "" {
		return c.core.Auth().HTTPClient(*source.AccountID, scopes)
	}
	if c.core.extensionID != "contacts" {
		return nil, coreapi.ErrCapabilityDenied
	}
	token, err := c.app.getValidContactSourceOAuthToken(id)
	if err != nil {
		return nil, err
	}
	stored, err := c.app.credStore.GetContactSourceOAuthTokens(id)
	if err != nil {
		return nil, err
	}
	for _, scope := range scopes {
		if !stored.HasScope(scope.Resource) {
			return nil, &coreapi.ErrAdditionalConsentRequired{MissingScopes: scopes, Reason: "contact grant is missing or unknown; authorize this contact source separately"}
		}
	}
	return davutil.NewBearerHTTPClient(token, 30*time.Second), nil
}

func (c contactsCoreImpl) davClient(source *carddav.Source) (*carddav.Client, error) {
	if source.AccountID != nil && *source.AccountID != "" {
		httpClient, err := c.sourceHTTPClient(source.ID, []coreapi.AuthScope{{Resource: "carddav", Reason: "Write contacts to your CardDAV server"}})
		if err != nil {
			return nil, err
		}
		return carddav.NewClientWithHTTPClient(davutil.NewWebDAVClient(httpClient.Transport, 30*time.Second), source.URL)
	}
	password, err := carddav.NewPasswordStore(c.app.db.DB, c.app.credStore).GetPassword(source.ID)
	if err != nil {
		return nil, err
	}
	return carddav.NewClient(source.URL, source.Username, password)
}
