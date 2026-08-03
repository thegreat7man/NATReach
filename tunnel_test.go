package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func TestParseEndpoint(t *testing.T) {
	ev, ok := parseEndpoint("ready: tcp://example.pinggy.link:54321")
	if !ok || ev.Host != "example.pinggy.link" || ev.Port != 54321 {
		t.Fatalf("unexpected parse result: %#v, %v", ev, ok)
	}
	if _, ok := parseEndpoint("tcp://bad:99999"); ok {
		t.Fatal("accepted invalid port")
	}
}

func TestEndToEndSSHIntegration(t *testing.T) {
	if os.Getenv("NATREACH_INTEGRATION") != "1" {
		t.Skip("set NATREACH_INTEGRATION=1 to use the public tunnel")
	}
	password := randomPassword()
	logFn := func(message string) { t.Log(message) }
	server, err := StartSSH(SSHOptions{Username: "remote", Password: password, Log: logFn})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	ready := make(chan TunnelEvent, 1)
	tunnel := NewTunnel(server.Port(), func(event TunnelEvent) {
		if event.Kind == "ready" {
			select {
			case ready <- event:
			default:
			}
		}
	}, logFn)
	go tunnel.Run(ctx)
	var endpoint TunnelEvent
	select {
	case endpoint = <-ready:
	case <-ctx.Done():
		t.Fatal("public endpoint was not allocated")
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", endpoint.Host, endpoint.Port), &ssh.ClientConfig{
		User: "remote", Auth: []ssh.AuthMethod{ssh.Password(password)}, HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: 15 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	output, err := session.CombinedOutput("printf natreach-ok")
	if err != nil {
		t.Fatalf("remote command failed: %v (%s)", err, output)
	}
	if string(output) != "natreach-ok" {
		t.Fatalf("unexpected output: %q", output)
	}
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		t.Fatal(err)
	}
	defer sftpClient.Close()
	if _, err := sftpClient.ReadDir("."); err != nil {
		t.Fatalf("SFTP listing failed: %v", err)
	}
}

func TestTunnelIntegration(t *testing.T) {
	if os.Getenv("NATREACH_INTEGRATION") != "1" {
		t.Skip("set NATREACH_INTEGRATION=1 to use the public tunnel")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	tunnel := NewTunnel(1, func(event TunnelEvent) { t.Logf("event: %#v", event) }, func(message string) { t.Log(message) })
	ready, err := tunnel.connectOnce(ctx)
	t.Logf("result ready=%v err=%v", ready, err)
	if !ready {
		t.Fatal("tunnel did not become ready")
	}
}
