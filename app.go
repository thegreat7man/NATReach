package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os/user"
	"runtime"
	"sync"
	"time"
)

type StartOptions struct {
	Admin bool `json:"admin"`
}

type Status struct {
	State       string   `json:"state"`
	Message     string   `json:"message"`
	Endpoint    string   `json:"endpoint"`
	Command     string   `json:"command"`
	Username    string   `json:"username"`
	Password    string   `json:"password"`
	Fingerprint string   `json:"fingerprint"`
	SystemUser  string   `json:"systemUser"`
	Admin       bool     `json:"admin"`
	Elevated    bool     `json:"elevated"`
	Platform    string   `json:"platform"`
	StartedAt   string   `json:"startedAt"`
	Logs        []string `json:"logs"`
}

type App struct {
	mu        sync.RWMutex
	status    Status
	ssh       *SSHService
	tunnel    *Tunnel
	cancel    context.CancelFunc
	ready     chan struct{}
	readyOnce *sync.Once
}

func NewApp() *App {
	u, _ := user.Current()
	name := "unknown"
	if u != nil && u.Username != "" {
		name = u.Username
	}
	return &App{
		status: Status{State: "stopped", Message: "Off", SystemUser: name, Platform: runtime.GOOS, Elevated: isElevated()},
	}
}

func (a *App) Start(opts StartOptions) error {
	a.mu.Lock()
	if a.status.State != "stopped" && a.status.State != "error" {
		a.mu.Unlock()
		return errors.New("NATReach is already running")
	}
	if runtime.GOOS == "windows" && opts.Admin && !isElevated() {
		a.mu.Unlock()
		return ErrElevationRequired
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.ready = make(chan struct{})
	a.readyOnce = &sync.Once{}
	a.status = Status{
		State: "starting", Message: "Starting local SSH...", Username: "remote", Password: randomPassword(),
		SystemUser: a.status.SystemUser, Platform: runtime.GOOS, Admin: opts.Admin, Elevated: isElevated(), StartedAt: time.Now().Format(time.RFC3339),
	}
	password := a.status.Password
	a.mu.Unlock()

	sshService, err := StartSSH(SSHOptions{Username: "remote", Password: password, Admin: opts.Admin, Log: a.addLog})
	if err != nil {
		cancel()
		a.setError(fmt.Sprintf("SSH failed to start: %v", err))
		return err
	}
	a.mu.Lock()
	a.ssh = sshService
	a.status.Fingerprint = sshService.Fingerprint()
	a.status.Message = "Connecting the secure TCP tunnel..."
	a.mu.Unlock()

	tunnel := NewTunnel(sshService.Port(), a.onTunnelEvent, a.addLog)
	a.mu.Lock()
	a.tunnel = tunnel
	a.mu.Unlock()
	go tunnel.Run(ctx)
	return nil
}

func (a *App) onTunnelEvent(event TunnelEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.status.State == "stopped" {
		return
	}
	switch event.Kind {
	case "ready":
		a.status.State = "running"
		a.status.Message = "SSH is available over the internet"
		a.status.Endpoint = event.Endpoint
		a.status.Command = fmt.Sprintf("ssh -p %d %s@%s", event.Port, a.status.Username, event.Host)
		if a.readyOnce != nil {
			a.readyOnce.Do(func() { close(a.ready) })
		}
	case "reconnecting":
		a.status.State = "reconnecting"
		a.status.Message = event.Message
		a.status.Endpoint = ""
		a.status.Command = ""
	case "error":
		a.status.State = "error"
		a.status.Message = event.Message
	}
}

func (a *App) Stop() {
	a.mu.Lock()
	cancel := a.cancel
	sshService := a.ssh
	a.cancel = nil
	a.ssh = nil
	a.tunnel = nil
	a.status.State = "stopped"
	a.status.Message = "Off - port closed"
	a.status.Endpoint = ""
	a.status.Command = ""
	a.status.Password = ""
	a.status.Fingerprint = ""
	a.status.Logs = appendLimited(a.status.Logs, time.Now().Format("15:04:05")+"  All connections stopped")
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if sshService != nil {
		_ = sshService.Close()
	}
}

func (a *App) WaitReady(ctx context.Context) error {
	a.mu.RLock()
	ready := a.ready
	a.mu.RUnlock()
	if ready == nil {
		return errors.New("not started")
	}
	select {
	case <-ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *App) Status() Status {
	a.mu.RLock()
	defer a.mu.RUnlock()
	s := a.status
	s.Logs = append([]string(nil), a.status.Logs...)
	return s
}

func (a *App) addLog(message string) {
	a.mu.Lock()
	a.status.Logs = appendLimited(a.status.Logs, time.Now().Format("15:04:05")+"  "+message)
	a.mu.Unlock()
}

func (a *App) setError(message string) {
	a.mu.Lock()
	a.status.State = "error"
	a.status.Message = message
	a.status.Logs = appendLimited(a.status.Logs, time.Now().Format("15:04:05")+"  "+message)
	a.mu.Unlock()
}

func appendLimited(items []string, value string) []string {
	items = append(items, value)
	if len(items) > 80 {
		items = items[len(items)-80:]
	}
	return items
}

func randomPassword() string {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
