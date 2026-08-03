//go:build !windows

package main

import (
	"fmt"
	"os"
)

func isElevated() bool { return os.Geteuid() == 0 }

func requestElevation() error {
	return fmt.Errorf("restarting the entire application as root is unnecessary: administrator mode runs sudo inside SSH")
}

func prepareConsole() {}
