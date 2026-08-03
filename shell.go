package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/Kodecable/crosspty"
	gossh "github.com/gliderlabs/ssh"
)

func runShell(session gossh.Session, admin bool) error {
	ptyReq, winCh, hasPTY := session.Pty()
	rows, cols := uint16(24), uint16(80)
	term := "xterm-256color"
	if hasPTY {
		if ptyReq.Window.Height > 0 {
			rows = uint16(ptyReq.Window.Height)
		}
		if ptyReq.Window.Width > 0 {
			cols = uint16(ptyReq.Window.Width)
		}
		if ptyReq.Term != "" {
			term = ptyReq.Term
		}
	}
	argv, err := shellCommand(session.RawCommand(), admin)
	if err != nil {
		return err
	}
	p, err := crosspty.Start(crosspty.CommandConfig{
		Argv:      argv,
		Env:       os.Environ(),
		EnvInject: map[string]string{"TERM": term, "NATREACH": "1"},
		Size:      crosspty.TermSize{Rows: rows, Cols: cols},
	})
	if err != nil {
		return fmt.Errorf("could not open the terminal: %w", err)
	}
	defer p.Close()

	if hasPTY {
		go func() {
			for w := range winCh {
				_ = p.Resize(crosspty.TermSize{Rows: uint16(w.Height), Cols: uint16(w.Width)})
			}
		}()
	}

	go func() {
		_, _ = io.Copy(p, session)
	}()
	_, _ = io.Copy(session, p)
	exitCode := p.Wait()
	_ = p.Close()
	if exitCode < 0 {
		exitCode = 0
	}
	return session.Exit(exitCode)
}

func shellCommand(raw string, admin bool) ([]string, error) {
	if runtime.GOOS == "windows" {
		shell := "powershell.exe"
		if raw == "" {
			return []string{shell, "-NoLogo"}, nil
		}
		return []string{shell, "-NoLogo", "-NoProfile", "-Command", raw}, nil
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	if admin {
		if raw == "" {
			return []string{"sudo", "-i"}, nil
		}
		return []string{"sudo", shell, "-lc", raw}, nil
	}
	if raw == "" {
		return []string{shell, "-l"}, nil
	}
	return []string{shell, "-lc", strings.TrimSpace(raw)}, nil
}
