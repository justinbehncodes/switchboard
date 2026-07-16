# Security & privacy

Switchboard does things that, pattern-wise, look like what browser hijackers
do: it registers as a browser, writes registry keys, and runs a native
messaging host. This document explains exactly what it touches and why, so
you can verify every claim against the source.

## What it writes, and why

| What | Where | Why | Removed by |
|---|---|---|---|
| Browser registration (ProgID, StartMenuInternet client, RegisteredApplications) | `HKCU\Software\Classes\Switchboard.URL`, `HKCU\Software\Clients\StartMenuInternet\Switchboard`, `HKCU\Software\RegisteredApplications` | So Switchboard appears as a choosable browser in Settings > Default apps | `switchboard uninstall` |
| Autostart entry (opt-in) | `HKCU\...\CurrentVersion\Run` | Starts the tray icon at login, only if you enable "Start with Windows" | Toggling it off |
| Config and log | `%LOCALAPPDATA%\Switchboard\` | Your rules and the routing log | Deleting the folder |

Everything is per-user (HKCU). Nothing requires admin. Windows itself
gatekeeps the actual "default browser" choice — Switchboard can only open
the Settings page; you make the click.

## What data goes where

- **Nothing leaves your machine.** No telemetry, no update checks, no
  network calls except serving its own UI on `127.0.0.1`.
- The **routing log** (`%LOCALAPPDATA%\Switchboard\route.log`) records
  timestamp, source app, URL, and chosen profile — with the URL's query
  string and fragment stripped, because they can carry tokens.
- Registered URL handling is restricted to `http`/`https`, and arguments
  that are not web URLs are rejected — a crafted argument can't smuggle
  extra browser switches.

## Verifying a release

Releases are built by the public GitHub Actions workflow in this repo.
Each release ships a `SHA256SUMS` file, and binaries carry GitHub build
provenance — verify that a download came from this source with:

```
gh attestation verify switchboard.exe -R justinbehncodes/switchboard
```

## Reporting a vulnerability

Open a GitHub security advisory on this repository (Security tab >
"Report a vulnerability"), or open an issue if it's not sensitive.
