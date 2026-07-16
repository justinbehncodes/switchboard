package winutil

import (
	"golang.org/x/sys/windows/registry"
)

// Registration is per-user (HKCU), needs no admin rights, and is fully
// reversible via Uninstall. Windows deliberately does not let a program make
// itself the default browser; after Install the user picks Switchboard once
// in Settings > Default apps.
const (
	progID       = "Switchboard.URL"
	classKeyPath = `Software\Classes\Switchboard.URL`
	clientPath   = `Software\Clients\StartMenuInternet\Switchboard`
	regAppsPath  = `Software\RegisteredApplications`
	appName      = "Switchboard"
)

func setValues(path string, values map[string]string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	for name, val := range values {
		if err := k.SetStringValue(name, val); err != nil {
			return err
		}
	}
	return nil
}

// Install registers exe as a browser candidate for http/https.
func Install(exe string) error {
	steps := []struct {
		path   string
		values map[string]string
	}{
		{classKeyPath, map[string]string{"": "Switchboard URL Router"}},
		{classKeyPath + `\DefaultIcon`, map[string]string{"": exe + ",0"}},
		{classKeyPath + `\shell\open\command`, map[string]string{"": `"` + exe + `" "%1"`}},
		{clientPath, map[string]string{"": appName}},
		{clientPath + `\Capabilities`, map[string]string{
			"ApplicationName":        appName,
			"ApplicationDescription": "Routes links to the right browser profile based on where they were clicked and where they point.",
		}},
		{clientPath + `\Capabilities\URLAssociations`, map[string]string{
			"http":  progID,
			"https": progID,
		}},
		{clientPath + `\shell\open\command`, map[string]string{"": `"` + exe + `" ui`}},
		{regAppsPath, map[string]string{appName: clientPath + `\Capabilities`}},
	}
	for _, s := range steps {
		if err := setValues(s.path, s.values); err != nil {
			return err
		}
	}
	NotifyAssociationsChanged()
	return nil
}

// Uninstall removes everything Install wrote.
func Uninstall() error {
	if k, err := registry.OpenKey(registry.CURRENT_USER, regAppsPath, registry.SET_VALUE); err == nil {
		k.DeleteValue(appName)
		k.Close()
	}
	for _, path := range []string{classKeyPath, clientPath} {
		if err := deleteTree(path); err != nil {
			return err
		}
	}
	NotifyAssociationsChanged()
	return nil
}

func deleteTree(path string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, path, registry.ENUMERATE_SUB_KEYS)
	if err == registry.ErrNotExist {
		return nil
	}
	if err != nil {
		return err
	}
	subs, err := k.ReadSubKeyNames(-1)
	k.Close()
	if err != nil {
		return err
	}
	for _, sub := range subs {
		if err := deleteTree(path + `\` + sub); err != nil {
			return err
		}
	}
	return registry.DeleteKey(registry.CURRENT_USER, path)
}

// Installed reports whether the browser registration exists.
func Installed() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, regAppsPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(appName)
	return err == nil
}

const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

// AutostartEnabled reports whether Switchboard's tray starts at login.
func AutostartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(appName)
	return err == nil
}

// SetAutostart adds or removes the per-user Run entry that launches the tray
// at login.
func SetAutostart(exe string, enabled bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if enabled {
		return k.SetStringValue(appName, `"`+exe+`" tray`)
	}
	if err := k.DeleteValue(appName); err != nil && err != registry.ErrNotExist {
		return err
	}
	return nil
}

// SystemPrefersDark reports whether Windows apps are set to dark mode.
func SystemPrefersDark() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return true
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	if err != nil {
		return true
	}
	return v == 0
}

// IsDefaultBrowser reports whether the user has picked Switchboard as the
// https handler in Settings > Default apps. Windows 11 24H2+ records the
// choice under UserChoiceLatest and can leave the legacy UserChoice key
// stale, so the new location wins when present.
func IsDefaultBrowser() bool {
	const base = `Software\Microsoft\Windows\Shell\Associations\UrlAssociations\https\`
	for _, sub := range []string{`UserChoiceLatest\ProgId`, `UserChoice`} {
		k, err := registry.OpenKey(registry.CURRENT_USER, base+sub, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		v, _, err := k.GetStringValue("ProgId")
		k.Close()
		if err == nil {
			return v == progID
		}
	}
	return false
}
