# Release Steps

## 1. Bump the version

1. `app/state.go` - Go app version
2. `frontend/package.json` - npm package version
3. `frontend/package-lock.json` - auto-updates with npm install
4. `build/flatpak/io.github.caseylabs.MailPong.metainfo.xml` - Flatpak version and release date
5. `CHANGELOG.md` - add new release entry
6. `wails.json` - product version

## 2. Dry run (optional, recommended)

Actions > Release > Run workflow, on the branch to check, with `dry_run`
left on (the default). It builds every platform and the Flatpak bundles and
uploads them as workflow artifacts. It never creates a release.

## 3. Tag and review the draft

1. Tag the release commit and push the tag: `git tag v0.4.0 && git push origin v0.4.0`.
2. The Release workflow builds everything and creates a **draft prerelease**
   named `MailPong vX.Y.Z`. Nothing is published and no step pushes to a branch.
3. Review the draft's assets: `mailpong-vX.Y.Z-linux-{x86_64,aarch64}`,
   `mailpong-linux-{amd64,arm64}.tar.gz`, `MailPong-darwin-{amd64,arm64}.zip`,
   `MailPong-windows-{amd64,arm64}.exe`, `MailPong-windows-setup-{amd64,arm64}.exe`
   and `MailPong-vX.Y.Z-{x86_64,aarch64}.flatpak`.
4. Edit the notes, then publish the release manually from the GitHub UI.

Release builds ship no OAuth credentials; users bring their own.

## 4. Flathub manifest (when submitting)

Pin the from-source manifest with `build/flatpak/flathub/calculate-hashes.sh vX.Y.Z`
and submit the change through a pull request.
