// Package winutil wraps the Win32 calls Switchboard needs.
package winutil

import (
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	kernel32                     = windows.NewLazySystemDLL("kernel32.dll")
	shell32                      = windows.NewLazySystemDLL("shell32.dll")
	dwmapi                       = windows.NewLazySystemDLL("dwmapi.dll")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procMessageBoxW              = user32.NewProc("MessageBoxW")
	procAttachConsole            = kernel32.NewProc("AttachConsole")
	procSHChangeNotify           = shell32.NewProc("SHChangeNotify")
	procDwmSetWindowAttribute    = dwmapi.NewProc("DwmSetWindowAttribute")
)

// SetDarkTitleBar switches a window's native title bar to dark, matching the
// system theme (DWMWA_USE_IMMERSIVE_DARK_MODE).
func SetDarkTitleBar(hwnd uintptr, dark bool) {
	var v int32
	if dark {
		v = 1
	}
	const dwmwaUseImmersiveDarkMode = 20
	procDwmSetWindowAttribute.Call(hwnd, dwmwaUseImmersiveDarkMode, uintptr(unsafe.Pointer(&v)), 4)
}

// ForegroundProcessName returns the lowercase executable name (e.g.
// "slack.exe") of the process owning the foreground window. Call this as the
// very first thing in main: the app the user clicked a link in is only
// guaranteed to still be foreground at process start. Returns "" on failure.
func ForegroundProcessName() string {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return ""
	}
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return ""
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, 1024)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return ""
	}
	return strings.ToLower(filepath.Base(windows.UTF16ToString(buf[:size])))
}

// AttachParentConsole reattaches stdout/stderr to the invoking terminal.
// The binary is built with -H windowsgui (no console flash on link clicks),
// which detaches it from any console; CLI subcommands call this so their
// output is visible when run from a shell. Handles that are already valid
// (e.g. a pipe from the shell) are left alone.
func AttachParentConsole() {
	const attachParentProcess = ^uintptr(0)
	procAttachConsole.Call(attachParentProcess)
	if !validStdHandle(windows.STD_OUTPUT_HANDLE) {
		if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
			os.Stdout = f
		}
	}
	if !validStdHandle(windows.STD_ERROR_HANDLE) {
		if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
			os.Stderr = f
		}
	}
}

func validStdHandle(which uint32) bool {
	h, err := windows.GetStdHandle(which)
	return err == nil && h != 0 && h != windows.InvalidHandle
}

// MessageBox shows a blocking error/info dialog (used when there is no
// console and no webview to report through).
func MessageBox(title, text string) {
	t, _ := windows.UTF16PtrFromString(title)
	x, _ := windows.UTF16PtrFromString(text)
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(x)), uintptr(unsafe.Pointer(t)), 0x40 /*MB_ICONINFORMATION*/)
}

// NotifyAssociationsChanged tells the shell that URL associations changed so
// Settings > Default apps picks up the registration without a logoff.
func NotifyAssociationsChanged() {
	const shcneAssocChanged = 0x08000000
	procSHChangeNotify.Call(shcneAssocChanged, 0, 0, 0)
}

// OpenDefaultAppsSettings deep-links to Switchboard's own page in
// Settings > Default apps, one click away from "Set default".
func OpenDefaultAppsSettings() {
	verb, _ := windows.UTF16PtrFromString("open")
	uri, _ := windows.UTF16PtrFromString("ms-settings:defaultapps?registeredAppUser=Switchboard")
	windows.ShellExecute(0, verb, uri, nil, nil, windows.SW_SHOWNORMAL)
}
