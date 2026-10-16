-- Mailven schema v61 -> legacy Aerion schema v30.
-- Derived from rollback-v39-to-v30.sql; original reconstruction preserved below.
-- Quit all app/composer processes and back up the database plus credential files
-- before use. This loses submission/Sent evidence (including duplicate guards),
-- sender caches, chat preferences/draft associations, account stable identity,
-- trust exceptions and newer preferences, mail API routes, identities, queued
-- API actions and migration records, plus legacy contact losses below.
-- This targets v61 only. Run with sqlite3 -bail so errors stop before COMMIT.
BEGIN TRANSACTION;

-- v61–v58: remove mail API routes, identity, queued actions and migrations.
DROP TABLE api_migrations;
DROP TABLE api_pending_actions;
DROP TABLE api_sync_state;
DROP TABLE api_message_rows;
DROP TABLE api_message_refs;
DROP TABLE account_mail_api_routes;

-- v57–v40: remove newer local evidence/preferences and added columns first.
DROP TABLE submission_copies;
DROP TABLE sent_policies;
DROP TABLE submission_attempts;
DROP TRIGGER thread_starters_insert;
DROP TRIGGER thread_starters_delete;
DROP TRIGGER thread_starters_update;
DROP TRIGGER thread_starters_update_moved;
DROP INDEX idx_messages_folder_thread_key;
DROP TABLE thread_starters;
DROP TABLE account_send_as;
DROP TABLE chat_drafts;
DROP TABLE sender_chat;
DROP TABLE sender_category;
DROP TABLE conversation_state;
ALTER TABLE accounts DROP COLUMN last_sync_error_folder;
ALTER TABLE accounts DROP COLUMN last_sync_error;
ALTER TABLE messages DROP COLUMN pinned;
ALTER TABLE messages DROP COLUMN delivered_to;
ALTER TABLE identities DROP COLUMN reply_to;
ALTER TABLE attachments DROP COLUMN embedded;
ALTER TABLE messages DROP COLUMN is_bulk;
-- v30 trusted fingerprints were global. Clear trust instead of broadening a
-- host-specific exception during downgrade; the older app will prompt again.
DROP TABLE trusted_certificates;
CREATE TABLE trusted_certificates (
 id TEXT PRIMARY KEY, fingerprint TEXT NOT NULL UNIQUE, host TEXT NOT NULL DEFAULT '',
 subject TEXT NOT NULL, issuer TEXT NOT NULL, not_before DATETIME, not_after DATETIME,
 accepted_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
ALTER TABLE smime_sender_certs DROP COLUMN trusted;
ALTER TABLE drafts DROP COLUMN reply_to;
ALTER TABLE accounts DROP COLUMN imap_auth_mechanism;
ALTER TABLE accounts DROP COLUMN smtp_auth_mechanism;
ALTER TABLE accounts DROP COLUMN oauth_stable_id;


-- 1. Recreate legacy `contacts` table with the v30 / pre-v0.3.0 shape.
--    Older Aerion's contact.Store.ensureTable created this table with five
--    columns only (no name_overridden, no kind). Migration 31 was updated
--    to backfill name_overridden / kind from literal defaults rather than
--    selecting them from this table, so there's no longer any reason to
--    preserve those columns through the rollback — they wouldn't survive
--    re-upgrade anyway (migration 31 reruns after rollback clears the
--    migrations tracker at step 8).
CREATE TABLE contacts (
    email TEXT PRIMARY KEY,
    display_name TEXT,
    send_count INTEGER DEFAULT 0,
    last_used DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_contacts_send_count ON contacts(send_count DESC);
CREATE INDEX idx_contacts_last_used ON contacts(last_used DESC);

-- 2. Restore local-contact rows. One row per (record, email) pair where the
--    record is sourced locally. Round-trip is lossless for email / name /
--    send_count / last_used. The kind (manual vs collected) and the
--    name_overridden flag are dropped: the v30 schema has no place to
--    store them, and re-upgrading would substitute literal defaults
--    regardless of what the rollback preserved.
INSERT INTO contacts (email, display_name, send_count, last_used, created_at)
SELECT
    ce.email,
    cr.fn,
    ce.send_count,
    ce.last_used,
    cr.created_at
FROM contact_records cr
JOIN contact_emails ce ON ce.record_id = cr.id
WHERE cr.source = 'local';

-- 3. Recreate legacy `carddav_contacts` table with v30 schema.
CREATE TABLE carddav_contacts (
    id TEXT PRIMARY KEY,
    addressbook_id TEXT NOT NULL,
    email TEXT NOT NULL,
    display_name TEXT,
    href TEXT,
    etag TEXT,
    synced_at DATETIME,
    FOREIGN KEY (addressbook_id) REFERENCES contact_source_addressbooks(id) ON DELETE CASCADE
);

CREATE INDEX idx_carddav_contacts_addressbook ON carddav_contacts(addressbook_id);
CREATE INDEX idx_carddav_contacts_email ON carddav_contacts(email);

-- 4. Restore carddav-contact rows. Re-introduces the fan-out: one row per
--    (record, email) pair. Synthetic ID `<record_id>:<email>` ensures uniqueness;
--    older Aerion matches contacts on next sync via (addressbook_id, href).
INSERT INTO carddav_contacts (id, addressbook_id, email, display_name, href, etag, synced_at)
SELECT
    cr.id || ':' || ce.email,
    crs.addressbook_id,
    ce.email,
    cr.fn,
    crs.href,
    crs.etag,
    crs.synced_at
FROM contact_records cr
JOIN contact_emails ce ON ce.record_id = cr.id
JOIN carddav_record_state crs ON crs.record_id = cr.id
WHERE cr.source = 'carddav';

-- 5. Drop the unified tables. Multi-field data is gone after this — same
--    semantics as removing columns from a v30 install that never had them.
DROP TABLE carddav_record_state;
DROP TABLE contact_categories;
DROP TABLE contact_impps;
DROP TABLE contact_urls;
DROP TABLE contact_addresses;
DROP TABLE contact_phones;
DROP TABLE contact_emails;
DROP TABLE contact_records;

-- 6. Drop the v35 extension_secrets table. Any extension secret values stored
--    in this table (encrypted ciphertext) are lost. Keyring-stored entries
--    are NOT touched by this SQL — clear them via the OS keyring manager
--    (Seahorse / Keychain / Credential Manager) if you want a full cleanup.
DROP TABLE IF EXISTS extension_secrets;

-- 7. Roll back v36's per-(account, client_config) OAuth fallback.
--    7a. Delete extension-slot oauth_tokens rows. v0.2.5 only knows about the
--        mail slots; leaving extension rows behind would also leak past v36's
--        purpose. Per-slot keyring entries from these rows (keyed as
--        "<accountID>:<configID>:access_token" / refresh_token) are NOT cleared
--        by this SQL — clear them via the OS keyring manager if desired.
DELETE FROM oauth_tokens
 WHERE client_config_id NOT IN ('google-mail', 'microsoft-mail');

--    7b. Drop the encrypted fallback columns added in v36. SQLite supports
--        DROP COLUMN since 3.35 — modernc.org/sqlite (Aerion's driver) is well
--        past that. If you're on a stock CLI older than 3.35 these will fail;
--        upgrade sqlite3 or skip these two statements (the columns being
--        present is harmless to v0.2.5 — v0.2.5 just ignores unknown columns
--        — but leaving them in place will cause v36's ADD COLUMN to fail on
--        the next upgrade).
ALTER TABLE oauth_tokens DROP COLUMN encrypted_access_token;
ALTER TABLE oauth_tokens DROP COLUMN encrypted_refresh_token;

-- 8. Roll back v37's per-account "No outgoing server" + separate SMTP
--    credentials columns. Same DROP COLUMN caveat as step 7b — needs SQLite
--    >= 3.35. The columns being present is harmless to v0.2.5 (it ignores
--    unknown columns), but leaving them in place will cause v37's ADD COLUMN
--    to fail on the next upgrade. Separate-SMTP keyring entries (keyed as
--    "<accountID>:smtp") are NOT cleared by this SQL — clear them via the
--    OS keyring manager if desired. v0.2.5 doesn't look at them, so the
--    orphaned entries are inert until you upgrade again.
ALTER TABLE accounts DROP COLUMN no_outgoing_server;
ALTER TABLE accounts DROP COLUMN smtp_username;
ALTER TABLE accounts DROP COLUMN encrypted_smtp_password;

-- 9. Roll back v38's "Reply/Forward with" identity preference column. Same
--    DROP COLUMN caveat as step 7b — needs SQLite >= 3.35. The column being
--    present is harmless to v0.2.5 (it ignores unknown columns), but
--    leaving it in place will cause v38's ADD COLUMN to fail on the next
--    upgrade. No keyring or external state to clean up — the column is a
--    pure FK reference to identities.id.
ALTER TABLE accounts DROP COLUMN reply_forward_identity_id;

-- 10. Roll back v39's persistent body-parse-failed flag on messages. Same
--     DROP COLUMN caveat — needs SQLite >= 3.35. v0.2.5 ignores the column
--     if left in place, but leaving it causes v39's ADD COLUMN to fail on
--     re-upgrade. No external state attached — the column was the entire
--     persistence layer for the v39 fix; any message previously marked
--     unparseable will re-enter v0.2.5's old (unbounded) retry behavior,
--     which is the state the user was in before the v0.3.0 fix.
ALTER TABLE messages DROP COLUMN body_failed;

-- 11. Roll back the migration tracker so older Aerion doesn't think v31..v39
--     have been applied. After this, older Aerion sees schema_version=30 and
--     starts normally. The `>= 31` bound catches all v0.3.0 migrations plus
--     any future intermediate schemas.
DELETE FROM migrations WHERE version >= 31;

COMMIT;

-- Verify (optional, run separately to confirm):
--   sqlite3 aerion.db "SELECT COUNT(*) FROM contacts; SELECT COUNT(*) FROM carddav_contacts; SELECT MAX(version) FROM migrations;"
