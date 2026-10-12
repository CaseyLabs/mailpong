# Security Policy

## Reporting a Vulnerability

Mailven is alpha software maintained by volunteers. There is **no response-time guarantee**.

Do not post vulnerability details, credentials, or private mail data in public issues. Use GitHub private vulnerability reporting: open the **Security** tab of [caseylabs/mailven](https://github.com/CaseyLabs/mailven/security) and choose **Report a vulnerability**.

Include the affected version or commit, impact, reproduction steps, and a minimal proof of concept with sensitive data removed. State whether the issue also reproduces on upstream Aerion, if known.

For vulnerabilities that exist only in upstream Aerion, follow [upstream's security policy](https://github.com/hkdb/aerion/security/policy). Upstream's contact address and response commitments apply to upstream, not to Mailven.

## Security Best Practices for Users

### OAuth Credentials

If you are compiling Mailven from source:

1. **Never commit OAuth credentials** to version control
2. Use the `.env.example` file as a template and create your own `.env` file
3. Ensure `.env` is listed in `.gitignore` (it is by default)
4. Rotate your OAuth credentials periodically
5. Use separate OAuth applications for development and production

### Email Security

- Mailven stores emails locally on your device
- Use strong passwords for your email accounts
- Enable 2FA/MFA on your email accounts where possible
- For Gmail/Google Workspace: Use App-Specific Passwords or OAuth

### Data Storage

- Email data is stored in SQLite databases in your local data directory
- Ensure your device has appropriate security measures (disk encryption, screen lock, etc.)
- Back up your data regularly

## Security Features

Mailven includes the following security measures:

- **HTML Sanitization**: All HTML email content is sanitized before display to prevent XSS attacks
- **OAuth 2.0**: Secure authentication for Gmail and other OAuth-supporting providers
- **Local Storage**: Emails are stored locally, not on third-party servers
- **TLS/SSL**: All IMAP/SMTP connections use TLS encryption

## Known Limitations

- Mailven is currently in early stage active development
- Use at your own risk for sensitive communications
