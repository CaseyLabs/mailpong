package credentials

import "fmt"

// RequireMailAPICapability checks recorded grant evidence, not token validity,
// mailbox delegation rights, tenant policy or delivery. Unlike legacy protocol
// checks, unknown grants never authorize a new API route. Operations are read,
// write, send and aliases (Gmail send-as list). Shared is another mailbox.
func (t *OAuthTokens) RequireMailAPICapability(operation string, shared bool) error {
	if t == nil || !t.ScopesKnown {
		return fmt.Errorf("mail API authorization is unknown; authorize the API route explicitly")
	}
	var resource string
	var scopes []string
	switch t.Provider {
	case "google":
		resource = "https://www.googleapis.com"
		if shared {
			return fmt.Errorf("Gmail delegated-mailbox API access is unsupported")
		}
		switch operation {
		case "read":
			scopes = []string{"https://www.googleapis.com/auth/gmail.readonly", "https://www.googleapis.com/auth/gmail.modify"}
		case "write":
			scopes = []string{"https://www.googleapis.com/auth/gmail.modify"}
		case "send":
			scopes = []string{"https://www.googleapis.com/auth/gmail.send", "https://www.googleapis.com/auth/gmail.compose", "https://www.googleapis.com/auth/gmail.modify"}
		case "aliases":
			scopes = []string{"https://www.googleapis.com/auth/gmail.settings.basic", "https://www.googleapis.com/auth/gmail.modify", "https://www.googleapis.com/auth/gmail.readonly"}
		default:
			return fmt.Errorf("unsupported mail API operation %q", operation)
		}
		// Full-mail grants also permit these ordinary API operations. Do not
		// request them in the new profile just to gain permanent deletion.
		scopes = append(scopes, "https://mail.google.com/")
	case "microsoft":
		resource = "https://graph.microsoft.com"
		switch operation {
		case "read":
			scopes = []string{"Mail.Read", "Mail.ReadWrite"}
		case "write":
			scopes = []string{"Mail.ReadWrite"}
		case "send":
			scopes = []string{"Mail.Send"}
		default:
			return fmt.Errorf("unsupported mail API operation %q", operation)
		}
		var qualified []string
		for _, scope := range scopes {
			if !shared {
				qualified = append(qualified, resource+"/"+scope)
			}
			// Delegated Shared permissions include the user's own mailbox.
			qualified = append(qualified, resource+"/"+scope+".Shared")
		}
		scopes = qualified
	default:
		return fmt.Errorf("mail API access is unsupported for provider %q", t.Provider)
	}
	if t.ScopeResource != resource {
		return fmt.Errorf("mail API authorization has missing or incorrect resource; authorize the API route explicitly")
	}
	for _, scope := range scopes {
		if t.HasScope(scope) {
			return nil
		}
	}
	return fmt.Errorf("mail API authorization does not include %s access (shared=%t); grant the missing permission", operation, shared)
}
