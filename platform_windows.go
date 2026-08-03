//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/sys/windows"
)

func isElevated() bool { return windows.GetCurrentProcessToken().IsElevated() }

func requestElevation() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	quoted := strings.ReplaceAll(exe, "'", "''")
	command := fmt.Sprintf("Start-Process -FilePath '%s' -Verb RunAs", quoted)
	return exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", command).Start()
}

func prepareConsole() {
	_ = windows.SetConsoleOutputCP(65001)
	_ = windows.SetConsoleCP(65001)
}
