# Flathub Submission (deferred)

Mailven is not on Flathub, and submission is deferred until after the first
public release. This directory holds the from-source manifest so it can be
tested and, later, submitted.

## Files

- `io.github.caseylabs.Mailven.yml` - From-source manifest for Flathub (vendored Go modules and npm packages, no bundled OAuth credentials)
- `io.github.caseylabs.Mailven-source.yml` - Alternative manifest that builds the local checkout (not used for Flathub)
- `go.mod.yml`, `modules.txt` - Vendored Go module sources
- `node-sources.json`, `gen-node-sources.py` - Vendored npm package sources
- `calculate-hashes.sh` - Pins the manifest to a release tag and commit
- `release.sh` - Copies the manifest and vendored sources into a Flathub repository checkout
- `../io.github.caseylabs.Mailven.metainfo.xml` - AppStream metadata
- `../../linux/io.github.caseylabs.Mailven.desktop` - Desktop file
- `../../linux/icons/` - Application icons

## Preparing a release

1. Tag the release (for example `v0.4.0`) and push the tag to `caseylabs/mailven`.
2. Pin the manifest: `./calculate-hashes.sh v0.4.0`. It resolves the tag's commit and
   rewrites `tag:` and `commit:` in `io.github.caseylabs.Mailven.yml`.
3. Regenerate `go.mod.yml`, `modules.txt` and `node-sources.json` if dependencies changed.
4. Test: `../test-build.sh v0.4.0` (Docker) or `../build-local.sh`.

## Submitting

Follow the [Flathub submission guide](https://github.com/flathub/flathub/wiki/App-Submission).
Flathub requires a verifiable app ID: `io.github.caseylabs.Mailven` is verified through the
`caseylabs` GitHub account. After Flathub creates `flathub/io.github.caseylabs.Mailven`, copy the files with
`./release.sh /path/to/flathub/io.github.caseylabs.Mailven` and open a pull request there.
