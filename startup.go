package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	appName = "WindowsThemeTray"
	runKey  = `Software\Microsoft\Windows\CurrentVersion\Run`
	// Task Manager's "Startup apps" tab stores per-entry enabled/disabled state here.
	startupApprovedKey = `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`
)

// startupEnabled reports whether Windows will actually launch this executable at
// sign-in: the Run entry must point at the current exe and must not be disabled
// in Task Manager.
func startupEnabled() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	cmd, _, err := k.GetStringValue(appName)
	if err != nil {
		return false
	}
	if !strings.EqualFold(strings.Trim(strings.TrimSpace(cmd), `"`), exe) {
		return false
	}
	return !disabledInTaskManager()
}

func disabledInTaskManager() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, startupApprovedKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	state, _, err := k.GetBinaryValue(appName)
	if err != nil || len(state) == 0 {
		return false
	}
	// Even values (0x02, 0x06) mean enabled, odd values (0x03, 0x07) disabled.
	return state[0]&1 != 0
}

func setStartup(enable bool) error {
	if enable {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("locate executable: %w", err)
		}
		k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
		if err != nil {
			return fmt.Errorf("open Run key: %w", err)
		}
		defer k.Close()
		if err := k.SetStringValue(appName, `"`+exe+`"`); err != nil {
			return fmt.Errorf("write Run entry: %w", err)
		}
		// Clear a "disabled" flag left by Task Manager, otherwise the entry is ignored.
		return deleteValue(startupApprovedKey, appName)
	}

	if err := deleteValue(runKey, appName); err != nil {
		return err
	}
	return deleteValue(startupApprovedKey, appName)
}

// deleteValue removes a HKCU value; a missing key or value is not an error.
func deleteValue(path, name string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, path, registry.SET_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer k.Close()

	if err := k.DeleteValue(name); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("delete %s: %w", name, err)
	}
	return nil
}
