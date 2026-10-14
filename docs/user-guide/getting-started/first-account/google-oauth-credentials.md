---
sidebar_position: 4
---

# Custom Google OAuth Credentials

Mailven ships with no Google OAuth credentials, so Gmail OAuth, the Calendar extension and the Contacts extension all need an OAuth client that you create. (Gmail also works without OAuth, using an [app password](/docs/getting-started/first-account/gmail).) This guide walks you through creating a Google Cloud project and using its OAuth Client ID and Secret for **everything in one shot** — Gmail, the [Calendar extension](/docs/features/extensions/calendar), and the [Contacts extension](/docs/features/extensions/contacts).

:::note Why you need this
- Your mail, calendar, and contacts all flow through your own Google Cloud project, which you control.
- The redirect used by the sign-in flow is a local `http://localhost` address, which a Desktop app client allows automatically.
:::

## 1. Create a Google Cloud Project

1. Go to the [Google Cloud Console](https://console.cloud.google.com) and sign in with your Google account
2. Click the **project picker** at the top of the page (next to the Google Cloud logo)
3. Click **New Project**
4. Enter a project name (e.g. "Mailven") — the organization/location can be left as-is for personal accounts
5. Click **Create**
6. Once created, make sure the new project is selected in the project picker

## 2. Enable the APIs

Mailven starts Gmail accounts on IMAP/SMTP and offers an explicit switch to the Gmail API. Enable the APIs for the features you intend to use in this project:

1. In the console, go to **APIs & Services → Library**
2. Search for and **Enable** each of the following:
   - **Gmail API**
   - **Google Calendar API**
   - **Google People API** (used for contacts)

## 3. Configure the OAuth Consent Screen

1. Go to **APIs & Services → OAuth consent screen** (Google may also call this **Google Auth Platform**)
2. If prompted, click **Get started** and fill in:
   - **App name**: e.g. "Mailven"
   - **User support email**: your email address
   - **Audience**: **External**
   - **Contact information**: your email address
3. Agree to the policy and click **Create**

### Publish the App

Under **Audience**, click **Publish app** to move the app from **Testing** to **In production**.

:::note Why publish?
For external projects requesting Gmail access, Google expires refresh tokens issued in **Testing** after seven days. Moving to **In production** avoids that Testing-specific expiry; it does not complete OAuth verification. See [Google's token lifecycle](https://developers.google.com/identity/protocols/oauth2#expiration).
:::

If the project has not completed the applicable Google verification, you may see a "This app isn't verified" warning when authorizing — this is the same warning described in the [Gmail Setup Security Note](/docs/getting-started/first-account/gmail#security-note), except this time the unverified app is your own project. Click **Advanced → Go to Mailven (unsafe) → Allow** to proceed.

You can also add your own Google accounts as test users under **Audience → Test users**.

## 4. Add the Scopes

1. Go to **APIs & Services → OAuth consent screen → Data Access** (or **Google Auth Platform → Data Access**)
2. Click **Add or remove scopes**
3. Add the following scopes:

| Scope | Used For |
|-------|----------|
| `https://mail.google.com/` | Full Gmail access over IMAP/SMTP (Mail) |
| `https://www.googleapis.com/auth/gmail.modify` | Optional Gmail API route; authorized separately when preparing migration |
| `https://www.googleapis.com/auth/contacts.readonly` | Contact sync for autocomplete (Mail) and reading contacts (Contacts extension) |
| `https://www.googleapis.com/auth/contacts.other.readonly` | "Other contacts" for email autocomplete (Mail) |
| `https://www.googleapis.com/auth/contacts` | Editing contacts — only needed if you enable write access in the Contacts extension |
| `https://www.googleapis.com/auth/calendar` | Reading and writing calendar events (Calendar extension) |
| `https://www.googleapis.com/auth/userinfo.email` | Identifying which account you authorized |
| `openid` | OpenID Connect sign-in |

4. Click **Update**, then **Save**

## 5. Create the OAuth Client ID and Secret

1. Go to **APIs & Services → Credentials** (or **Google Auth Platform → Clients**)
2. Click **Create Credentials → OAuth client ID** (or **Create client**)
3. For **Application type**, select **Desktop app**
4. Enter a name (e.g. "Mailven Desktop") and click **Create**
5. Copy the **Client ID** and **Client Secret** shown — you'll paste both into Mailven next

## 6. Configure Mailven — Mail

1. In Mailven, open **Settings → Accounts** (tab)
2. Scroll to the bottom where **OAuth Credentials** is located and expand it
3. Under **Google Mail**, select **Custom** from the dropdown menu
4. Copy and paste the **Client ID** and **Client Secret** into their respective fields and save

If you already added your Gmail account, re-authorize it so it starts using your credentials: **Settings → Accounts → Edit** on the account → **Re-authorize**. Otherwise just [add your account with OAuth](/docs/getting-started/first-account/gmail) as usual.

## 7. Configure the Calendar Extension

Extensions are disabled by default, so enable it first:

1. Go to **Settings → Extensions** (tab) and toggle **Calendar** on
2. Click the **Edit** button of the extension
3. Scroll down to **OAuth Credentials (advanced)** and expand it
4. Under **Google**, select **Custom** from the dropdown menu
5. Copy and paste the same **Client ID** and **Client Secret** there too, and save

## 8. Configure the Contacts Extension

Repeat the same steps for the Contacts extension:

1. Go to **Settings → Extensions** (tab) and toggle **Contacts** on
2. Click the **Edit** button of the extension
3. Scroll down to **OAuth Credentials (advanced)** and expand it
4. Under **Google**, select **Custom** from the dropdown menu
5. Copy and paste the same **Client ID** and **Client Secret**, and save

## Done

Mail, Calendar, and Contacts now all authenticate through your own Google Cloud project. Note that each of them keeps its own authorization — the first time each one connects to Google after the switch, you'll be asked to sign in and consent again (including the unverified-app warning described above).
