# Changelog

## Unreleased

Initial release.

- Router: registers per-user as a browser; routes links by source app
  (slack.exe, ms-teams.exe, …) and URL glob to Edge/Chrome profiles.
- Settings UI (WebView2): profiles, reorderable rules, fallback, test box,
  activity log; follows system light/dark theme.
- Picker popup for unmatched links with "always use this profile for this
  site".
- Unwraps google.com/url and Outlook SafeLinks redirectors before matching.
- Tray icon with optional start-at-login.
- CLI: install, uninstall, doctor, test, version.
