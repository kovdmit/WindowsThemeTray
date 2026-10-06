package main

import (
	"golang.org/x/sys/windows"
)

const (
	hotkeyID   = 1
	hotkeyName = "Ctrl+Alt+D"

	modAlt      = 0x0001
	modControl  = 0x0002
	modNoRepeat = 0x4000
	vkD         = 'D'
)

// registerToggleHotkey registers Ctrl+Alt+D system-wide; WM_HOTKEY is delivered
// to hwnd regardless of which application is in the foreground.
func registerToggleHotkey(hwnd windows.HWND) error {
	r, _, err := procRegisterHotKey.Call(uintptr(hwnd), hotkeyID, modControl|modAlt|modNoRepeat, vkD)
	if r == 0 {
		return err
	}
	return nil
}

func unregisterToggleHotkey(hwnd windows.HWND) {
	procUnregisterHotKey.Call(uintptr(hwnd), hotkeyID)
}
