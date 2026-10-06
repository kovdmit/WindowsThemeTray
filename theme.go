package main

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const personalizeKey = `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`

type theme int

const (
	themeLight theme = iota
	themeDark
)

func (t theme) String() string {
	if t == themeDark {
		return "Dark"
	}
	return "Light"
}

func (t theme) opposite() theme {
	if t == themeDark {
		return themeLight
	}
	return themeDark
}

// currentTheme reads the app theme from the registry. A missing key or value
// means Windows uses its default, which is Light.
func currentTheme() (theme, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, personalizeKey, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return themeLight, nil
	}
	if err != nil {
		return themeLight, fmt.Errorf("open %s: %w", personalizeKey, err)
	}
	defer k.Close()

	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	if errors.Is(err, registry.ErrNotExist) {
		return themeLight, nil
	}
	if err != nil {
		return themeLight, fmt.Errorf("read AppsUseLightTheme: %w", err)
	}
	if v == 0 {
		return themeDark, nil
	}
	return themeLight, nil
}

// setTheme switches both apps and system (taskbar, Start) to t.
func setTheme(t theme) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, personalizeKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open %s: %w", personalizeKey, err)
	}
	defer k.Close()

	var light uint32
	if t == themeLight {
		light = 1
	}
	for _, name := range []string{"AppsUseLightTheme", "SystemUsesLightTheme"} {
		if err := k.SetDWordValue(name, light); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}

// broadcastThemeChange sends the same notification the Settings app sends, so
// running applications re-read the theme immediately. It may block for a while
// on unresponsive windows, so callers should run it off the UI thread.
func broadcastThemeChange() {
	param, err := windows.UTF16PtrFromString("ImmersiveColorSet")
	if err != nil {
		return
	}
	var result uintptr
	procSendMessageTimeoutW.Call(
		hwndBroadcast,
		wmSettingChange,
		0,
		uintptr(unsafe.Pointer(param)),
		smtoAbortIfHung,
		1000,
		uintptr(unsafe.Pointer(&result)),
	)
}
