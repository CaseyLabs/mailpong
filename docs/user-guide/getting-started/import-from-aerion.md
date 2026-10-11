---
sidebar_position: 3
---

# Import from Aerion

MailPong starts with empty data and never reads Aerion's data on its own. If you used Aerion before, you can copy your data across by hand. This is optional, manual, and leaves Aerion untouched: you **copy** the files and never move them.

:::warning Read before you start
- Quit **both** MailPong and Aerion first. Copying a database while an app has it open can corrupt the copy.
- Do not run Aerion and MailPong at the same time against the same accounts while you test.
- Passwords and OAuth tokens are stored in your system keyring under the service name `aerion`. MailPong does not read them, and you should not copy or reuse Aerion's keyring secrets. You will enter passwords again or re-authorize OAuth accounts in MailPong.
:::

## Where the data lives

| Platform | Aerion | MailPong |
|----------|--------|----------|
| Linux config | `~/.config/aerion` | `~/.config/mailpong` |
| Linux data | `~/.local/share/aerion` (database `aerion.db`) | `~/.local/share/mailpong` (database `mailpong.db`) |
| Linux cache | `~/.cache/aerion` | `~/.cache/mailpong` |
| macOS data and config | `~/Library/Application Support/Aerion` | `~/Library/Application Support/MailPong` |
| macOS cache | `~/Library/Caches/Aerion` | `~/Library/Caches/MailPong` |
| Windows data and config | `%APPDATA%\Aerion` | `%APPDATA%\MailPong` |
| Windows cache | `%LOCALAPPDATA%\Aerion\Cache` | `%LOCALAPPDATA%\MailPong\Cache` |
| Flatpak | `~/.var/app/<aerion-flatpak-id>/` (`config/aerion`, `data/aerion`, `cache/aerion`) | `~/.var/app/io.github.caseylabs.MailPong/` (`config/mailpong`, `data/mailpong`, `cache/mailpong`) |

The Aerion Flatpak ID is `io.github.hkdb.Aerion` for upstream releases, or `io.github.clabs_public_forks.Aerion` for the earlier fork.

The cache can be rebuilt, so you only need the config and data directories.

## Steps (Linux shown)

1. **Quit both apps** completely.
2. **Back up the Aerion data** as a dated archive, and keep it until you are sure you no longer need it:

   ```bash
   tar -czf ~/aerion-backup-$(date +%F).tar.gz ~/.config/aerion ~/.local/share/aerion
   ```

3. **Copy** (never move) the contents into **empty** MailPong directories, then rename the database. Start with MailPong's directories absent or empty; if MailPong has already created data you want to keep, do not use this procedure.

   ```bash
   mkdir -p ~/.config/mailpong ~/.local/share/mailpong
   cp -a ~/.config/aerion/. ~/.config/mailpong/
   cp -a ~/.local/share/aerion/. ~/.local/share/mailpong/
   cd ~/.local/share/mailpong
   for f in aerion.db aerion.db-wal aerion.db-shm; do
     [ -e "$f" ] && mv "$f" "mailpong${f#aerion}"
   done
   ```

   On macOS and Windows, copy the same contents between the directories in the table, then rename `aerion.db` (and `aerion.db-wal` and `aerion.db-shm` if present) to `mailpong.db` (and the matching `-wal` and `-shm` names).

4. **Start MailPong.** Your accounts, folders and messages appear, but sign-in credentials are missing: re-enter each account's password, or open **Settings → Accounts → Edit** on an OAuth account and choose **Re-authorize**. You also need your own OAuth credentials; see [Custom Google OAuth Credentials](./first-account/google-oauth-credentials.md).

5. **Check it worked** before you delete anything: open a few accounts and messages, and send a test email.

## Rolling back

Quit MailPong and delete its directories (`~/.config/mailpong`, `~/.local/share/mailpong`, `~/.cache/mailpong`, or the equivalents in the table). Aerion's data was only copied, so it is unchanged. Remove the `mailpong` entries from your keyring if you want a clean slate.

## Things that do not carry over

- Keyring secrets (passwords, OAuth tokens, PGP and S/MIME passphrases), as described above.
- Settings that name another app, such as "start on login" entries. Set them again in MailPong.
- MailPong cannot be pointed back at a database after Aerion has written newer schema changes to it. If a copied database comes from a newer Aerion than this MailPong knows, MailPong refuses to open it.
