// Command WindowsThemeTray lives in the notification area and switches the
// Windows Light/Dark theme with a single click or Ctrl+Alt+D.
package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Session-local so that every signed-in user gets their own instance.
const singleInstanceMutex = `Local\WindowsThemeTray-SingleInstance`

var errAlreadyRunning = errors.New("already running")

func init() {
	// The window, tray icon, hotkey and message loop must share one OS thread.
	runtime.LockOSThread()
}

func main() {
	if err := run(); err != nil {
		showError(err)
		os.Exit(1)
	}
}

func run() error {
	release, err := acquireSingleInstance()
	if errors.Is(err, errAlreadyRunning) {
		return nil
	}
	if err != nil {
		return err
	}
	defer release()

	enableDPIAwareness()

	t, err := newTray()
	if err != nil {
		return err
	}
	defer t.icons.destroy()

	return runMessageLoop()
}

func acquireSingleInstance() (release func(), err error) {
	name, err := windows.UTF16PtrFromString(singleInstanceMutex)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		windows.CloseHandle(h)
		return nil, errAlreadyRunning
	}
	if err != nil {
		return nil, fmt.Errorf("CreateMutex: %w", err)
	}
	return func() { windows.CloseHandle(h) }, nil
}

// enableDPIAwareness makes Windows give us real icon metrics instead of
// bitmap-scaling a 16px icon on high-DPI displays.
func enableDPIAwareness() {
	if procSetProcessDpiAwarenessContext.Find() == nil {
		if r, _, _ := procSetProcessDpiAwarenessContext.Call(dpiAwarenessContextPerMonitorAwareV2); r != 0 {
			return
		}
	}
	procSetProcessDPIAware.Call()
}

func runMessageLoop() error {
	var m msg
	for {
		r, _, err := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		switch int32(r) {
		case -1:
			return fmt.Errorf("GetMessage: %w", err)
		case 0:
			return nil
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func showError(err error) {
	title, _ := windows.UTF16PtrFromString(appName)
	text, _ := windows.UTF16PtrFromString(err.Error())
	windows.MessageBox(0, text, title, windows.MB_OK|windows.MB_ICONERROR)
}
