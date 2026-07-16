# Switchboard — context-aware link router for Windows

A tiny Go program that registers as the default browser, then routes every
clicked link to the right browser **profile** based on (a) the URL and (b) the
app the link was clicked in.

**Status:** built and verified — router, registry install/uninstall, doctor,
settings UI (WebView2), picker popup with "remember this site", and tray
icon with start-at-login. See README.md for usage.

## Design goals

- **Links land in the right context automatically.** Work links open in the
  work profile, everything else stays personal — no more copying URLs
  between browser windows or ending up signed into the wrong account.
- **Rules as versioned config.** A single TOML file that can live in a
  dotfiles repo, diffable and restorable on a new machine — vs. clicking
  through a settings UI.
- **Tiny and inspectable.** One static Go binary, no installer, no updater,
  no telemetry. Everything it does to the registry is in one `install`
  function you can read.

## Architecture

One executable, `switchboard.exe`:

1. **Route (default):** `switchboard.exe <url>`
   - On the very first line of `main`, capture the foreground window
     (`GetForegroundWindow` → `GetWindowThreadProcessId` → process image
     name). The source app (slack.exe, ms-teams.exe, discord.exe,
     whatsapp.exe, signal.exe, telegram.exe, olk.exe…) is still foreground
     when Windows hands us the URL, so this reliably identifies where the
     click came from.
   - Unwrap known tracking redirectors (google.com/url, Outlook SafeLinks).
   - Load rules from `%LOCALAPPDATA%\Switchboard\config.toml`.
   - First matching rule wins; launch the mapped browser+profile via
     `--profile-directory`. No rule → fallback profile, or the picker popup
     when the fallback is `ask`.
   - Append one line to a rolling log: timestamp, source exe, URL (query
     string and fragment stripped — they can carry tokens), chosen target.

2. **Install:** `switchboard.exe install` — per-user registry keys (no
   admin, fully reversible): a ProgID, a `StartMenuInternet` client with
   `URLAssociations` for http/https, and a `RegisteredApplications` entry.
   Windows does not allow programs to *set* themselves default; after
   install the user picks Switchboard once in Settings > Default apps (the
   app deep-links straight to its own page there). `uninstall` removes
   exactly those keys.

3. **UI:** `switchboard.exe ui` — settings window (WebView2 + embedded HTML
   served on a loopback socket): profiles, reorderable rules, fallback,
   test box, activity log. Follows the system light/dark theme.

4. **Tray:** `switchboard.exe tray` — notification-area icon; left-click
   opens settings (separate process, so closing the window keeps the tray),
   right-click menu. Single-instance via named mutex. The UI's "Start with
   Windows" toggle adds a per-user Run entry.

## Config (example)

```toml
# %LOCALAPPDATA%\Switchboard\config.toml
[browsers]
edge   = 'C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe'
chrome = 'C:\Program Files\Google\Chrome\Application\chrome.exe'

[profiles.home]
browser = "chrome"
dir = "Default"

[profiles.work]
browser = "edge"
dir = "Profile 1"

# First match wins. `source` matches the exe the link was clicked in;
# `url` is a case-insensitive glob against the full URL.

[[rule]]
source = "slack.exe"    # any messenger works: ms-teams.exe, discord.exe,
profile = "work"        # whatsapp.exe, signal.exe, telegram.exe, …

[[rule]]
source = "ms-teams.exe"
profile = "work"

[[rule]]
source = "discord.exe"
profile = "home"

[[rule]]
url = "*mycompany.com*"
profile = "work"

[[rule]]
url = "*teams.microsoft.com*"
profile = "work"

[fallback]
profile = "home"   # or "ask" to get the picker for unmatched links
```

Always target the profile **directory name** (`Default`, `Profile 1`, …)
from the browser's `Local State`, never the display name — Chromium display
names and directory names diverge.

## Implementation notes

- Go, stdlib + `golang.org/x/sys` (registry + win32), `BurntSushi/toml`,
  `jchv/go-webview2` (no cgo). Built with `-H=windowsgui` so no console
  window flashes on link clicks; CLI subcommands reattach to the parent
  console (leaving already-valid pipe handles alone).
- Foreground-process capture must happen immediately at startup, before
  config load, to avoid racing focus changes.
- Only http/https URLs are registered to us; anything else is rejected so a
  crafted argument can't smuggle extra browser switches.
- Windows 11 24H2+ records the default-browser choice under
  `UserChoiceLatest` and can leave the legacy `UserChoice` key stale — check
  the new key first.

## Test plan

- Unit: rule matching (source, url globs, ordering, fallback), redirector
  unwrapping, config round-trip, profile discovery parsing.
- Manual: click links from Slack, Teams, Outlook, and a terminal; confirm
  routing + log lines; `doctor` after renaming a profile.
