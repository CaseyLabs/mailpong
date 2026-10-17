---
sidebar_position: 4
---

# Custom Google OAuth Credentials

Mailven ships with no Google OAuth credentials, so Gmail OAuth, the Calendar extension and the Contacts extension all need an OAuth client that you create. (Gmail also works without OAuth, using an [app password](/docs/getting-started/first-account/gmail).) This guide walks you through creating a Google Cloud project and using its OAuth Client ID and Secret for **everything in one shot** — Gmail, the [Calendar extension](/docs/features/extensions/calendar), and the [Contacts extension](/docs/features/extensions/contacts).

:::note Why you need this
- Your mail, calendar, and contacts all flow through your own Google Cloud project, which you control.
- The redirect used by the sign-in flow is a local `http://localhost` address, which a Desktop app client allows automatically.
:::

Mailven also has a **Setup checklist** with the same steps: in **Settings → Accounts → OAuth Credentials**, or on the Google sign-in form, press **Setup checklist**. It opens each console page in your browser, offers a copy button for the scope list and, on the sign-in form, an **Import JSON…** button, so you can finish without leaving the Add Account dialog.

:::note If a link opens the wrong page
Google moves its console pages from time to time. Each checklist step also names the menu path to follow from the console home page.
:::

## 1. Create a Google Cloud Project

1. Go to the [Google Cloud Console](https://console.cloud.google.com/projectcreate) and sign in with your Google account
2. Enter a project name (e.g. "Mailven") — the organization/location can be left as-is for personal accounts
3. Click **Create**
4. Once created, make sure the new project is selected in the project picker at the top of the page

## 2. Enable the APIs

Mailven starts Gmail accounts on IMAP/SMTP and offers an explicit switch to the Gmail API. Enable **Gmail API** for every Google OAuth account, even on IMAP/SMTP: sender review reads your Gmail "Send mail as" list through it, and without it sending from an address stays in review. Enable the other APIs for the features you intend to use in this project:

Use [this link](https://console.cloud.google.com/flows/enableapi?apiid=gmail.googleapis.com,people.googleapis.com,calendar-json.googleapis.com) to enable all three in one step, or go to **APIs & Services → Library** and enable each of the following:

- **Gmail API** (required for sender review; also used by the optional Gmail API route)
- **Google Calendar API**
- **Google People API** (used for contacts)

## 3. Set Up Branding

1. Go to **Google Auth Platform → Branding** ([link](https://console.cloud.google.com/auth/branding)). If prompted, click **Get started**
2. Fill in **App name** (e.g. "Mailven"), **User support email** and, under contact information, your email address
3. Agree to the policy and save

## 4. Choose the Audience

1. Go to **Google Auth Platform → Audience** ([link](https://console.cloud.google.com/auth/audience)) and select **External**
2. Decide between Testing and production, using the note below

### Publish the App

To publish, click **Publish app** to move the app from **Testing** to **In production**.

:::note Why publish?
For external projects requesting Gmail access, Google expires refresh tokens issued in **Testing** after seven days. Moving to **In production** avoids that Testing-specific expiry; it does not complete OAuth verification. See [Google's token lifecycle](https://developers.google.com/identity/protocols/oauth2#expiration).
:::

If the project has not completed the applicable Google verification, you may see a "This app isn't verified" warning when authorizing — this is the same warning described in the [Gmail Setup Security Note](/docs/getting-started/first-account/gmail#security-note), except this time the unverified app is your own project. Click **Advanced → Go to Mailven (unsafe) → Allow** to proceed.

You can also add your own Google accounts as test users under **Audience → Test users**.

## 5. Add the Scopes

1. Go to **Google Auth Platform → Data Access** ([link](https://console.cloud.google.com/auth/scopes)). The checklist's **Copy scopes** button copies this list
2. Click **Add or remove scopes**
3. Paste the scopes below into **Manually add scopes** at the bottom of the panel (one per line) and click **Add to table**, or tick them in the table:

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

## 6. Create the OAuth Client ID and Secret

1. Go to **Google Auth Platform → Clients** ([link](https://console.cloud.google.com/auth/clients))
2. Click **Create client**
3. For **Application type**, select **Desktop app**
4. Enter a name (e.g. "Mailven Desktop") and click **Create**
5. Click **Download JSON** now. Google shows the client secret only when the client is created, and you can import that file in the next step instead of pasting

## 7. Import the Client into Mailven

1. In Mailven, open **Settings → Accounts** (tab)
2. Scroll to the bottom where **OAuth Credentials** is located and expand it
3. Under **Google Mail**, select **Custom** from the dropdown menu
4. Copy and paste the **Client ID** and **Client Secret** into their respective fields and save

**Or import the downloaded file:** above the Google Mail editor (or in step 7 of the checklist on the Google sign-in form), click **Import JSON…**, tick the slots to fill (Google Mail, Google Contacts, Google Calendar — all ticked by default) and choose the file. Mailven reads the Desktop app client from it and saves it to every ticked slot at once, so steps 7 and 8 need no pasting. A file for any other client type (such as **Web application**), or one that is not valid Google JSON, is rejected with a message and changes nothing. Mailven never shows the secret after saving.

If you already added your Gmail account, re-authorize it so it starts using your credentials: **Settings → Accounts → Edit** on the account → **Re-authorize**. Otherwise just [add your account with OAuth](/docs/getting-started/first-account/gmail) as usual.

## 8. Sign In and Check the Setup

Sign in with your Google account, then open **Settings → Accounts → Edit** on it → **Identity** and press **Check Google setup**. It reports which of the Gmail, People and Calendar APIs work and offers **Enable API** or **Re-authorize** for any that do not.

## 9. Configure the Calendar Extension

Extensions are disabled by default, so enable it first:

1. Go to **Settings → Extensions** (tab) and toggle **Calendar** on
2. Click the **Edit** button of the extension
3. Scroll down to **OAuth Credentials (advanced)** and expand it
4. Under **Google**, select **Custom** from the dropdown menu
5. Copy and paste the same **Client ID** and **Client Secret** there too, and save

## 10. Configure the Contacts Extension

Repeat the same steps for the Contacts extension:

1. Go to **Settings → Extensions** (tab) and toggle **Contacts** on
2. Click the **Edit** button of the extension
3. Scroll down to **OAuth Credentials (advanced)** and expand it
4. Under **Google**, select **Custom** from the dropdown menu
5. Copy and paste the same **Client ID** and **Client Secret**, and save

## Done

Mail, Calendar, and Contacts now all authenticate through your own Google Cloud project. Note that each of them keeps its own authorization — the first time each one connects to Google after the switch, you'll be asked to sign in and consent again (including the unverified-app warning described above).
