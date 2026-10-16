package app

import (
	"fmt"
	"github.com/caseylabs/mailven/internal/account"
)

// oauthAccountDefaults is for new setup only. Empty Microsoft type is unknown;
// legacy API callers retain the prior tenant default without a domain inference.
func oauthAccountDefaults(provider, microsoftAccountType string) (account.AccountConfig, error) {
	config := account.AccountConfig{AuthType: account.AuthOAuth2, IMAPPort: 993, IMAPSecurity: account.SecurityTLS, SMTPPort: 587, SMTPSecurity: account.SecurityStartTLS, SyncPeriodDays: 30, SyncInterval: 30}
	switch provider {
	case "google":
		if microsoftAccountType != "" {
			return config, fmt.Errorf("Microsoft account type is only valid for Microsoft setup")
		}
		config.IMAPHost, config.SMTPHost = "imap.gmail.com", "smtp.gmail.com"
	case "microsoft":
		config.IMAPHost = "outlook.office365.com"
		switch microsoftAccountType {
		case "personal":
			config.SMTPHost = "smtp-mail.outlook.com"
		case "", "business":
			config.SMTPHost = "smtp.office365.com"
		default:
			return config, fmt.Errorf("choose a personal or work/school Microsoft account type")
		}
	default:
		return config, fmt.Errorf("unsupported OAuth provider: %s", provider)
	}
	return config, nil
}
