// Package tray runs Switchboard's notification-area icon: left-click opens
// the settings window (a separate process, so closing that window keeps the
// tray alive), right-click shows a menu. Single-instance via a named mutex.
package tray

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"switchboard/internal/winutil"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenuW         = user32.NewProc("AppendMenuW")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procShellNotifyIconW    = shell32.NewProc("Shell_NotifyIconW")
	procExtractIconW        = shell32.NewProc("ExtractIconW")
	procGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
)

const (
	wmDestroy   = 0x0002
	wmCommand   = 0x0111
	wmLButtonUp = 0x0202
	wmRButtonUp = 0x0205
	wmTray      = 0x8000 + 1 // WM_APP + 1

	nimAdd     = 0
	nimDelete  = 2
	nifMessage = 0x1
	nifIcon    = 0x2
	nifTip     = 0x4

	mfString    = 0x0
	mfSeparator = 0x800

	idOpen    = 1
	idDefault = 2
	idQuit    = 3
)

type wndClassEx struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type point struct{ X, Y int32 }

type winMsg struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

type notifyIconData struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         windows.GUID
	HBalloonIcon     uintptr
}

var nid notifyIconData

// Run blocks until the user quits from the menu. Returns immediately (nil)
// if another tray instance already owns the mutex.
func Run() error {
	name, _ := windows.UTF16PtrFromString(`Local\SwitchboardTray`)
	_, err := windows.CreateMutex(nil, false, name)
	if err == windows.ERROR_ALREADY_EXISTS {
		return nil
	}
	if err != nil {
		return err
	}

	hInst, _, _ := procGetModuleHandleW.Call(0)
	className, _ := windows.UTF16PtrFromString("SwitchboardTrayWnd")
	wc := wndClassEx{
		CbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     hInst,
		LpszClassName: className,
	}
	if atom, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		return errors.New("RegisterClassEx failed")
	}
	hwnd, _, _ := procCreateWindowExW.Call(
		0, uintptr(unsafe.Pointer(className)), 0, 0,
		0, 0, 0, 0, 0, 0, hInst, 0)
	if hwnd == 0 {
		return errors.New("CreateWindowEx failed")
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exeU16, _ := windows.UTF16PtrFromString(exe)
	hicon, _, _ := procExtractIconW.Call(hInst, uintptr(unsafe.Pointer(exeU16)), 0)

	nid = notifyIconData{
		CbSize:           uint32(unsafe.Sizeof(notifyIconData{})),
		HWnd:             hwnd,
		UID:              1,
		UFlags:           nifMessage | nifIcon | nifTip,
		UCallbackMessage: wmTray,
		HIcon:            hicon,
	}
	tip, _ := windows.UTF16FromString("Switchboard — link router")
	copy(nid.SzTip[:], tip)
	if ok, _, _ := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid))); ok == 0 {
		return errors.New("Shell_NotifyIcon failed")
	}

	var m winMsg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if r == 0 || int32(r) == -1 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	return nil
}

func wndProc(hwnd, message, wparam, lparam uintptr) uintptr {
	switch uint32(message) {
	case wmTray:
		switch uint32(lparam) {
		case wmLButtonUp:
			openSettings()
		case wmRButtonUp:
			showMenu(hwnd)
		}
		return 0
	case wmCommand:
		switch wparam & 0xffff {
		case idOpen:
			openSettings()
		case idDefault:
			winutil.OpenDefaultAppsSettings()
		case idQuit:
			procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
			procPostQuitMessage.Call(0)
		}
		return 0
	case wmDestroy:
		procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, message, wparam, lparam)
	return r
}

func showMenu(hwnd uintptr) {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	appendItem(menu, idOpen, "Open Switchboard")
	if !winutil.IsDefaultBrowser() {
		appendItem(menu, idDefault, "Make default browser…")
	}
	procAppendMenuW.Call(menu, mfSeparator, 0, 0)
	appendItem(menu, idQuit, "Quit")

	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	// Required so the menu closes when clicking elsewhere.
	procSetForegroundWindow.Call(hwnd)
	procTrackPopupMenu.Call(menu, 0, uintptr(pt.X), uintptr(pt.Y), 0, hwnd, 0)
	procDestroyMenu.Call(menu)
}

func appendItem(menu uintptr, id uintptr, label string) {
	l, _ := windows.UTF16PtrFromString(label)
	procAppendMenuW.Call(menu, mfString, id, uintptr(unsafe.Pointer(l)))
}

func openSettings() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	exec.Command(exe, "ui").Start()
}
