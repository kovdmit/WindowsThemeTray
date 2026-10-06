package main

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	trayIconID     = 1
	wmTrayCallback = wmApp + 1
	retryTimerID   = 1
	retryInterval  = 2000 // ms

	windowClassName = "WindowsThemeTrayWindow"
)

const (
	cmdLight = iota + 1
	cmdDark
	cmdToggle
	cmdStartup
	cmdExit
)

type tray struct {
	hwnd              windows.HWND
	icons             iconSet
	theme             theme
	hotkeyRegistered  bool
	msgTaskbarCreated uint32
}

// The window procedure is a plain callback, so it reaches the tray through this
// variable. All access happens on the locked UI thread.
var activeTray *tray

func newTray() (*tray, error) {
	icons, err := loadIcons()
	if err != nil {
		return nil, err
	}
	t := &tray{icons: icons}
	t.theme, _ = currentTheme() // falls back to Light if the registry is unreadable

	name, _ := windows.UTF16PtrFromString("TaskbarCreated")
	r, _, _ := procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(name)))
	t.msgTaskbarCreated = uint32(r)

	activeTray = t
	if err := t.createWindow(); err != nil {
		icons.destroy()
		return nil, err
	}

	// Explorer may not be ready yet when started at sign-in; keep retrying
	// until the icon is accepted (TaskbarCreated also triggers a re-add).
	if err := t.addIcon(); err != nil {
		procSetTimer.Call(uintptr(t.hwnd), retryTimerID, retryInterval, 0)
	}

	if err := registerToggleHotkey(t.hwnd); err != nil {
		t.notify(hotkeyName + " is already used by another application. Click the tray icon to switch the theme.")
	} else {
		t.hotkeyRegistered = true
	}
	return t, nil
}

func (t *tray) createWindow() error {
	var instance windows.Handle
	if err := windows.GetModuleHandleEx(0, nil, &instance); err != nil {
		return fmt.Errorf("GetModuleHandleEx: %w", err)
	}
	className, _ := windows.UTF16PtrFromString(windowClassName)

	wc := wndClassEx{
		lpfnWndProc:   windows.NewCallback(wndProc),
		hInstance:     instance,
		lpszClassName: className,
	}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	if atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		return fmt.Errorf("RegisterClassEx: %w", err)
	}

	// A hidden top-level window rather than a message-only one: only top-level
	// windows receive the TaskbarCreated and WM_SETTINGCHANGE broadcasts.
	hwnd, _, err := procCreateWindowExW.Call(
		wsExToolWindow,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(className)),
		wsPopup,
		0, 0, 0, 0,
		0, 0,
		uintptr(instance),
		0,
	)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowEx: %w", err)
	}
	t.hwnd = windows.HWND(hwnd)

	// Let TaskbarCreated through UIPI in case the app is run elevated.
	procChangeWindowMessageFilterEx.Call(hwnd, uintptr(t.msgTaskbarCreated), msgfltAllow, 0)
	return nil
}

func wndProc(hwnd, message, wParam, lParam uintptr) uintptr {
	t := activeTray
	if t == nil {
		r, _, _ := procDefWindowProcW.Call(hwnd, message, wParam, lParam)
		return r
	}

	switch message {
	case wmTrayCallback:
		// NOTIFYICON_VERSION_4: LOWORD(lParam) is the event,
		// wParam holds the anchor point in screen coordinates.
		switch lParam & 0xFFFF {
		case wmLButtonUp:
			t.toggle()
		case wmContextMenu:
			t.showMenu(int16(wParam&0xFFFF), int16(wParam>>16&0xFFFF))
		}
		return 0

	case wmHotkey:
		if wParam == hotkeyID {
			t.toggle()
		}
		return 0

	case wmSettingChange:
		if lParam != 0 && windows.UTF16PtrToString(*(**uint16)(unsafe.Pointer(&lParam))) == "ImmersiveColorSet" {
			t.syncWithRegistry()
		}

	case wmTimer:
		if wParam == retryTimerID && t.addIcon() == nil {
			procKillTimer.Call(hwnd, retryTimerID)
		}
		return 0

	case wmDestroy:
		t.removeIcon()
		procKillTimer.Call(hwnd, retryTimerID)
		if t.hotkeyRegistered {
			unregisterToggleHotkey(t.hwnd)
		}
		procPostQuitMessage.Call(0)
		return 0

	default:
		if t.msgTaskbarCreated != 0 && message == uintptr(t.msgTaskbarCreated) {
			t.onTaskbarCreated()
			return 0
		}
	}

	r, _, _ := procDefWindowProcW.Call(hwnd, message, wParam, lParam)
	return r
}

func (t *tray) toggle() {
	t.syncWithRegistry()
	t.apply(t.theme.opposite())
}

func (t *tray) apply(th theme) {
	if err := setTheme(th); err != nil {
		t.notify("Could not switch theme: " + err.Error())
		return
	}
	t.theme = th
	t.updateIcon()
	go broadcastThemeChange()
}

// syncWithRegistry picks up theme changes made outside the app (e.g. in Settings).
func (t *tray) syncWithRegistry() {
	th, err := currentTheme()
	if err != nil || th == t.theme {
		return
	}
	t.theme = th
	t.updateIcon()
}

func (t *tray) iconData(flags uint32) notifyIconData {
	d := notifyIconData{hWnd: t.hwnd, uID: trayIconID, uFlags: flags}
	d.cbSize = uint32(unsafe.Sizeof(d))
	return d
}

func (t *tray) tooltip() string {
	return "Windows Theme: " + t.theme.String()
}

func (t *tray) addIcon() error {
	d := t.iconData(nifMessage | nifIcon | nifTip | nifShowTip)
	d.uCallbackMessage = wmTrayCallback
	d.hIcon = windows.Handle(t.icons.forTheme(t.theme))
	copyUTF16(d.szTip[:], t.tooltip())

	// NIM_ADD can time out while Explorer is busy yet still succeed, so fall
	// back to NIM_MODIFY before reporting failure.
	if !shellNotifyIcon(nimAdd, &d) && !shellNotifyIcon(nimModify, &d) {
		return errors.New("Shell_NotifyIcon(NIM_ADD) failed")
	}
	d.uVersion = notifyIconVersion4
	if !shellNotifyIcon(nimSetVersion, &d) {
		return errors.New("Shell_NotifyIcon(NIM_SETVERSION) failed")
	}
	return nil
}

func (t *tray) updateIcon() {
	d := t.iconData(nifIcon | nifTip | nifShowTip)
	d.hIcon = windows.Handle(t.icons.forTheme(t.theme))
	copyUTF16(d.szTip[:], t.tooltip())
	shellNotifyIcon(nimModify, &d)
}

func (t *tray) removeIcon() {
	d := t.iconData(0)
	shellNotifyIcon(nimDelete, &d)
}

// onTaskbarCreated re-adds the icon after Explorer restarts. Icons are reloaded
// because this also happens after a DPI change.
func (t *tray) onTaskbarCreated() {
	if icons, err := loadIcons(); err == nil {
		old := t.icons
		t.icons = icons
		defer old.destroy()
	}
	if err := t.addIcon(); err != nil {
		procSetTimer.Call(uintptr(t.hwnd), retryTimerID, retryInterval, 0)
	}
}

func (t *tray) notify(text string) {
	d := t.iconData(nifInfo)
	d.dwInfoFlags = niifWarning
	copyUTF16(d.szInfoTitle[:], appName)
	copyUTF16(d.szInfo[:], text)
	shellNotifyIcon(nimModify, &d)
}

func (t *tray) showMenu(x, y int16) {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)

	t.syncWithRegistry()
	toggleLabel := "Toggle theme"
	if t.hotkeyRegistered {
		toggleLabel += "\t" + hotkeyName
	}

	appendMenuItem(menu, cmdLight, "☀ Light", t.theme == themeLight)
	appendMenuItem(menu, cmdDark, "🌙 Dark", t.theme == themeDark)
	appendMenuSeparator(menu)
	appendMenuItem(menu, cmdToggle, toggleLabel, false)
	appendMenuSeparator(menu)
	appendMenuItem(menu, cmdStartup, "Start with Windows", startupEnabled())
	appendMenuItem(menu, cmdExit, "Exit", false)
	// Bold "Toggle theme" hints that it is the left-click action.
	procSetMenuDefaultItem.Call(menu, cmdToggle, 0)

	// Without this the menu does not close when the user clicks elsewhere.
	procSetForegroundWindow.Call(uintptr(t.hwnd))

	flags := uintptr(tpmReturnCmd | tpmNoNotify | tpmRightButton)
	if getSystemMetrics(smMenuDropAlignment) != 0 {
		flags |= tpmRightAlign
	}
	cmd, _, _ := procTrackPopupMenuEx.Call(menu, flags, uintptr(x), uintptr(y), uintptr(t.hwnd), 0)

	// Recommended after TrackPopupMenu from a tray icon so that the next
	// invocation of the menu works correctly.
	procPostMessageW.Call(uintptr(t.hwnd), wmNull, 0, 0)

	t.runCommand(cmd)
}

func (t *tray) runCommand(cmd uintptr) {
	switch cmd {
	case cmdLight:
		t.apply(themeLight)
	case cmdDark:
		t.apply(themeDark)
	case cmdToggle:
		t.toggle()
	case cmdStartup:
		if err := setStartup(!startupEnabled()); err != nil {
			t.notify("Could not change startup setting: " + err.Error())
		}
	case cmdExit:
		procDestroyWindow.Call(uintptr(t.hwnd))
	}
}

func appendMenuItem(menu uintptr, id uintptr, label string, checked bool) {
	text, err := windows.UTF16PtrFromString(label)
	if err != nil {
		return
	}
	flags := uintptr(mfString)
	if checked {
		flags |= mfChecked
	}
	procAppendMenuW.Call(menu, flags, id, uintptr(unsafe.Pointer(text)))
}

func appendMenuSeparator(menu uintptr) {
	procAppendMenuW.Call(menu, mfSeparator, 0, 0)
}
