# Build Directory

The build directory is used to house all the build files and assets for your application. 

The structure is:

* bin - Output directory
* darwin - macOS specific files
* windows - Windows specific files

## Mac

The `darwin` directory holds files specific to Mac builds.
These may be customised and used as part of the build. To return these files to the default state, simply delete them
and
build with `make build`.

The directory contains the following files:

- `Info.plist` - the main plist file used for Mac builds. It is used when building using `make build`.
- `Info.dev.plist` - same as the main plist file but used when building using `make dev`.

## Windows

The `windows` directory contains the manifest and rc files used when building with `make build`.
These may be customised for your application. To return these files to the default state, simply delete them and
build with `make build`.

- `icon.ico` - The icon used for the application. This is used when building using `make build`. If you wish to
  use a different icon, simply replace this file with your own. If it is missing, a new `icon.ico` file
  will be created using the `appicon.png` file in the build directory.
- `installer/*` - The files used to create the Windows installer. These are used when building using `make build`.
- `info.json` - Application details used for Windows builds. The data here will be used by the Windows installer,
  as well as the application itself (right click the exe -> properties -> details)
- `wails.exe.manifest` - The main application manifest file.

## Mailven packaging identity

Mailven installs next to Aerion without sharing any of its state:

- macOS: bundle ID `io.github.caseylabs.Mailven`, app `mailven.app` (Wails names the bundle after
  the `wails.json` `name`; the Makefile derives the same spelling for case-sensitive volumes).
- Windows: install directory `Program Files\Mailven`, uninstall key `HKLM\Software\Microsoft\Windows\CurrentVersion\Uninstall\Mailven`
  (fixed by `UNINST_KEY_NAME` in `installer/project.nsi`), installer `build/bin/mailven-<arch>-installer.exe`.
- Code signing (Windows Authenticode) and macOS notarization are not set up yet; macOS builds are only
  ad-hoc signed. Both are future work.
