# Switchboard

[![CI](https://github.com/justinbehncodes/switchboard/actions/workflows/ci.yml/badge.svg)](https://github.com/justinbehncodes/switchboard/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Routes every link you click — in Slack, Teams, Discord, WhatsApp, Signal,
Telegram, Outlook, or any other messenger or app — to the right browser
**profile**, based on which app the link came from and where it points. Single Go binary, per-user install, no admin, no telemetry — see
[SECURITY.md](SECURITY.md) for exactly what it touches and why.

See [DESIGN.md](DESIGN.md) for the full design.

## Install

Grab `switchboard.exe` from the
[latest release](https://github.com/justinbehncodes/switchboard/releases)
(verify against `SHA256SUMS`), put it somewhere stable, then:

```
switchboard install     # registers it as a browser (per-user, reversible)
```

and pick **Switchboard** in the Settings page that opens. Scoop and
Chocolatey manifests live in [packaging/](packaging/) and will be published
once releases are signed.

## Building from source

```
make build          # bin/switchboard.exe
make install        # copies it to %LOCALAPPDATA%\Switchboard\, registers that
                    # copy as a browser + opens Default apps settings
```

Then in the Settings page that opens, set **Switchboard** as the default
browser. That's the one thing Windows makes you click yourself.

The app is a single portable exe — "installing" is just the copy to its
stable home plus per-user registry keys pointing at it. `make icon`
regenerates the embedded icon (assets/icon.ico + .syso resource).

## Use

- Click links anywhere — they open in the profile your rules say.
- `switchboard.exe` (or `ui`) — settings window: profiles, rules, fallback,
  test box, recent activity.
- Unmatched links open the fallback profile, or a picker popup if the
  fallback is set to `ask` (number keys / arrows + Enter; "always use this
  profile for this site" saves a rule).
- `switchboard.exe doctor` — sanity-check config vs. installed profiles.
- `switchboard.exe uninstall` — removes the registration (set another
  default browser afterwards).

## Tray & autostart

`switchboard.exe tray` puts an icon in the notification area: left-click
opens settings, right-click for a menu (open / make default / quit). The
settings window runs as its own process, so closing it keeps the tray alive
— reopen from the icon. The **Start with Windows** button in the UI adds a
per-user Run entry launching the tray at login (and starts it immediately).

## Config

`%LOCALAPPDATA%\Switchboard\config.toml` — created on first run from your
installed Edge/Chrome profiles. First matching rule wins; `source` matches
the exe the link was clicked in, `url` is a case-insensitive glob against
the full URL.

```toml
[[rule]]                  # links clicked in work messengers — works for any
source = "slack.exe"      # app: ms-teams.exe, discord.exe, whatsapp.exe,
profile = "work"          # signal.exe, telegram.exe, olk.exe, …

[[rule]]
source = "ms-teams.exe"
profile = "work"

[[rule]]                  # personal messengers → home profile
source = "discord.exe"
profile = "home"

[[rule]]
url = "*mycompany.com*"
profile = "work"

[fallback]
profile = "home"   # or "ask" for the picker
```

Routing decisions are logged (URL query strings stripped) to
`%LOCALAPPDATA%\Switchboard\route.log`.
