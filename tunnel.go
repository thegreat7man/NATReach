package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type TunnelEvent struct {
	Kind     string
	Message  string
	Endpoint string
	Host     string
	Port     int
}

type Tunnel struct {
	localPort int
	event     func(TunnelEvent)
	log       func(string)
}

var endpointPattern = regexp.MustCompile(`tcp://([A-Za-z0-9.-]+):([0-9]{1,5})`)

func NewTunnel(localPort int, event func(TunnelEvent), logFn func(string)) *Tunnel {
	return &Tunnel{localPort: localPort, event: event, log: logFn}
}

func (t *Tunnel) Run(ctx context.Context) {
	delay := time.Second
	first := true
	for {
		if ctx.Err() != nil {
			return
		}
		if !first {
			t.event(TunnelEvent{Kind: "reconnecting", Message: fmt.Sprintf("Tunnel reconnecting in %s...", delay)})
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}
		first = false
		ready, err := t.connectOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			t.log("Tunnel: " + friendlyTunnelError(err))
			if ready {
				delay = time.Second
			} else if delay < 15*time.Second {
				delay *= 2
			}
		}
	}
}

func (t *Tunnel) connectOnce(ctx context.Context) (bool, error) {
	t.log("Connecting to the free Pinggy TCP gateway...")
	dialer := net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
	netConn, err := dialer.DialContext(ctx, "tcp", "free.pinggy.io:443")
	if err != nil {
		return false, err
	}
	_ = netConn.SetDeadline(time.Now().Add(15 * time.Second))
	config := &ssh.ClientConfig{
		User:            "tcp",
		Auth:            []ssh.AuthMethod{ssh.Password("")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // The published stream contains a second, end-to-end authenticated SSH connection.
		Timeout:         15 * time.Second,
		ClientVersion:   "SSH-2.0-NATReach_" + version,
	}
	conn, chans, reqs, err := ssh.NewClientConn(netConn, "free.pinggy.io:443", config)
	if err != nil {
		_ = netConn.Close()
		return false, err
	}
	t.log("SSH connection to the gateway established")
	_ = netConn.SetDeadline(time.Time{})
	client := ssh.NewClient(conn, chans, reqs)
	defer client.Close()

	remote, err := client.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return false, fmt.Errorf("remote port: %w", err)
	}
	t.log("Gateway allocated a reverse port")
	defer remote.Close()
	go t.forwardLoop(ctx, remote)

	ev, err := t.fetchEndpoint(ctx, client)
	if err != nil {
		return false, fmt.Errorf("public address: %w", err)
	}
	ready := true
	t.event(ev)
	t.log("Public endpoint received: " + ev.Endpoint)
	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()
	waitCh := make(chan error, 1)
	go func() { waitCh <- client.Wait() }()
	for {
		select {
		case <-ctx.Done():
			return ready, ctx.Err()
		case err := <-waitCh:
			if err == nil {
				err = errors.New("gateway closed the connection")
			}
			return ready, err
		case <-keepalive.C:
			if _, _, err := client.SendRequest("keepalive@openssh.com", true, nil); err != nil {
				return ready, err
			}
		}
	}
}

func (t *Tunnel) fetchEndpoint(ctx context.Context, client *ssh.Client) (TunnelEvent, error) {
	deadline := time.Now().Add(12 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := client.Dial("tcp", "127.0.0.1:4300")
		if err == nil {
			_ = conn.SetDeadline(time.Now().Add(4 * time.Second))
			request, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost/urls", nil)
			request.Close = true
			if writeErr := request.Write(conn); writeErr == nil {
				response, readErr := http.ReadResponse(bufio.NewReader(conn), request)
				if readErr == nil {
					var payload struct {
						URLs []string `json:"urls"`
					}
					decodeErr := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&payload)
					_ = response.Body.Close()
					_ = conn.Close()
					if decodeErr == nil {
						for _, rawURL := range payload.URLs {
							if ev, ok := parseEndpoint(rawURL); ok {
								return ev, nil
							}
						}
						lastErr = errors.New("gateway returned no TCP address")
					} else {
						lastErr = decodeErr
					}
				} else {
					lastErr = readErr
					_ = conn.Close()
				}
			} else {
				lastErr = writeErr
				_ = conn.Close()
			}
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return TunnelEvent{}, ctx.Err()
		case <-time.After(350 * time.Millisecond):
		}
	}
	return TunnelEvent{}, lastErr
}

func (t *Tunnel) forwardLoop(ctx context.Context, remote net.Listener) {
	for {
		incoming, err := remote.Accept()
		if err != nil {
			return
		}
		go func() {
			defer incoming.Close()
			local, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", t.localPort))
			if err != nil {
				t.log("Could not forward the incoming connection to local SSH: " + err.Error())
				return
			}
			defer local.Close()
			var once sync.Once
			closeBoth := func() { _ = incoming.Close(); _ = local.Close() }
			go func() { _, _ = io.Copy(local, incoming); once.Do(closeBoth) }()
			_, _ = io.Copy(incoming, local)
			once.Do(closeBoth)
		}()
	}
}

func parseEndpoint(text string) (TunnelEvent, bool) {
	m := endpointPattern.FindStringSubmatch(text)
	if len(m) != 3 {
		return TunnelEvent{}, false
	}
	port, err := strconv.Atoi(m[2])
	if err != nil || port < 1 || port > 65535 {
		return TunnelEvent{}, false
	}
	return TunnelEvent{Kind: "ready", Endpoint: m[0], Host: m[1], Port: port}, true
}

func friendlyTunnelError(err error) string {
	text := err.Error()
	switch {
	case strings.Contains(text, "no such host"):
		return "DNS or internet access is unavailable; retrying automatically"
	case strings.Contains(text, "connection refused"):
		return "service is temporarily unavailable; retrying automatically"
	case strings.Contains(text, "unable to authenticate"):
		return "gateway rejected free authentication; retrying automatically"
	default:
		return text
	}
}
