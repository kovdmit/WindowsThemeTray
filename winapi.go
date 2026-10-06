package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32  = windows.NewLazySystemDLL("user32.dll")
	shell32 = windows.NewLazySystemDLL("shell32.dll")

	procAppendMenuW                   = user32.NewProc("AppendMenuW")
	procChangeWindowMessageFilterEx   = user32.NewProc("ChangeWindowMessageFilterEx")
	procCreateIconFromResourceEx      = user32.NewProc("CreateIconFromResourceEx")
	procCreatePopupMenu               = user32.NewProc("CreatePopupMenu")
	procCreateWindowExW               = user32.NewProc("CreateWindowExW")
	procDefWindowProcW                = user32.NewProc("DefWindowProcW")
	procDestroyIcon                   = user32.NewProc("DestroyIcon")
	procDestroyMenu                   = user32.NewProc("DestroyMenu")
	procDestroyWindow                 = user32.NewProc("DestroyWindow")
	procDispatchMessageW              = user32.NewProc("DispatchMessageW")
	procGetMessageW                   = user32.NewProc("GetMessageW")
	procGetSystemMetrics              = user32.NewProc("GetSystemMetrics")
	procKillTimer                     = user32.NewProc("KillTimer")
	procPostMessageW                  = user32.NewProc("PostMessageW")
	procPostQuitMessage               = user32.NewProc("PostQuitMessage")
	procRegisterClassExW              = user32.NewProc("RegisterClassExW")
	procRegisterHotKey                = user32.NewProc("RegisterHotKey")
	procRegisterWindowMessageW        = user32.NewProc("RegisterWindowMessageW")
	procSendMessageTimeoutW           = user32.NewProc("SendMessageTimeoutW")
	procSetForegroundWindow           = user32.NewProc("SetForegroundWindow")
	procSetMenuDefaultItem            = user32.NewProc("SetMenuDefaultItem")
	procSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procSetTimer                      = user32.NewProc("SetTimer")
	procTrackPopupMenuEx              = user32.NewProc("TrackPopupMenuEx")
	procTranslateMessage              = user32.NewProc("TranslateMessage")
	procUnregisterHotKey              = user32.NewProc("UnregisterHotKey")

	procShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
)

const (
	wmNull          = 0x0000
	wmDestroy       = 0x0002
	wmSettingChange = 0x001A
	wmContextMenu   = 0x007B
	wmTimer         = 0x0113
	wmLButtonUp     = 0x0202
	wmHotkey        = 0x0312
	wmApp           = 0x8000

	wsPopup         = 0x80000000
	wsExToolWindow  = 0x00000080
	hwndBroadcast   = 0xFFFF
	smtoAbortIfHung = 0x0002
	msgfltAllow     = 1

	smMenuDropAlignment = 40
	smCxSmIcon          = 49
	smCySmIcon          = 50

	mfString    = 0x0000
	mfChecked   = 0x0008
	mfSeparator = 0x0800

	tpmRightButton = 0x0002
	tpmRightAlign  = 0x0008
	tpmNoNotify    = 0x0080
	tpmReturnCmd   = 0x0100

	nimAdd        = 0
	nimModify     = 1
	nimDelete     = 2
	nimSetVersion = 4

	nifMessage  = 0x01
	nifIcon     = 0x02
	nifTip      = 0x04
	nifInfo     = 0x10
	nifShowTip  = 0x80
	niifWarning = 0x02

	notifyIconVersion4 = 4

	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 is defined as ((HANDLE)-4).
	dpiAwarenessContextPerMonitorAwareV2 = ^uintptr(3)
)

type point struct {
	x, y int32
}

type msg struct {
	hwnd     windows.HWND
	message  uint32
	wParam   uintptr
	lParam   uintptr
	time     uint32
	pt       point
	lPrivate uint32
}

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     windows.Handle
	hIcon         windows.Handle
	hCursor       windows.Handle
	hbrBackground windows.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       windows.Handle
}

type notifyIconData struct {
	cbSize           uint32
	hWnd             windows.HWND
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            windows.Handle
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         windows.GUID
	hBalloonIcon     windows.Handle
}

func shellNotifyIcon(message uint32, data *notifyIconData) bool {
	r, _, _ := procShellNotifyIconW.Call(uintptr(message), uintptr(unsafe.Pointer(data)))
	return r != 0
}

func getSystemMetrics(index int) int {
	r, _, _ := procGetSystemMetrics.Call(uintptr(index))
	return int(int32(r))
}

// copyUTF16 writes s into a fixed-size, NUL-terminated WCHAR buffer, truncating if needed.
func copyUTF16(dst []uint16, s string) {
	src, err := windows.UTF16FromString(s)
	if err != nil {
		src = []uint16{0}
	}
	n := copy(dst[:len(dst)-1], src)
	dst[n] = 0
}
