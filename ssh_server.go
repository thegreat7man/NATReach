package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	gossh "github.com/gliderlabs/ssh"
	"github.com/pkg/sftp"
	xssh "golang.org/x/crypto/ssh"
)

type SSHOptions struct {
	Username string
	Password string
	Admin    bool
	Log      func(string)
}

type SSHService struct {
	server      *gossh.Server
	listener    net.Listener
	fingerprint string
	port        int
	opts        SSHOptions
	closeOnce   sync.Once
}

func StartSSH(opts SSHOptions) (*SSHService, error) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("host key: %w", err)
	}
	signer, err := xssh.NewSignerFromKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("host signer: %w", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("local port: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	s := &SSHService{listener: listener, fingerprint: xssh.FingerprintSHA256(signer.PublicKey()), port: port, opts: opts}
	s.server = &gossh.Server{
		Handler: s.handleSession,
		PasswordHandler: func(ctx gossh.Context, password string) bool {
			userOK := subtle.ConstantTimeCompare([]byte(ctx.User()), []byte(opts.Username)) == 1
			passOK := subtle.ConstantTimeCompare([]byte(password), []byte(opts.Password)) == 1
			if !userOK || !passOK {
				time.Sleep(900 * time.Millisecond)
				return false
			}
			return true
		},
		PtyCallback: func(ctx gossh.Context, pty gossh.Pty) bool { return true },
		IdleTimeout: 2 * time.Hour,
		MaxTimeout:  12 * time.Hour,
		SubsystemHandlers: map[string]gossh.SubsystemHandler{
			"sftp": s.handleSFTP,
		},
		Version: "SSH-2.0-NATReach_" + version,
		Banner:  "NATReach: temporary session; close the application to disable access.\n",
	}
	s.server.AddHostKey(signer)
	go func() {
		err := s.server.Serve(listener)
		if err != nil && !errors.Is(err, gossh.ErrServerClosed) && opts.Log != nil {
			opts.Log("Local SSH stopped: " + err.Error())
		}
	}()
	if opts.Log != nil {
		opts.Log(fmt.Sprintf("Local SSH is listening only on 127.0.0.1:%d", port))
	}
	return s, nil
}

func (s *SSHService) Port() int           { return s.port }
func (s *SSHService) Fingerprint() string { return s.fingerprint }

func (s *SSHService) Close() error {
	var err error
	s.closeOnce.Do(func() {
		// Stop means stop: terminate active sessions as well as the listener.
		err = s.server.Close()
		_ = s.listener.Close()
	})
	return err
}

func (s *SSHService) handleSession(session gossh.Session) {
	if s.opts.Log != nil {
		s.opts.Log("SSH client connected")
		defer s.opts.Log("SSH client disconnected")
	}
	if err := runShell(session, s.opts.Admin); err != nil {
		_, _ = fmt.Fprintf(session.Stderr(), "NATReach: %v\r\n", err)
		_ = session.Exit(1)
	}
}

func (s *SSHService) handleSFTP(session gossh.Session) {
	server, err := sftp.NewServer(session)
	if err != nil {
		_, _ = fmt.Fprintf(session.Stderr(), "SFTP: %v\n", err)
		return
	}
	defer server.Close()
	if err := server.Serve(); err != nil && !errors.Is(err, sftp.ErrSSHFxEOF) && s.opts.Log != nil {
		s.opts.Log("SFTP ended: " + err.Error())
	}
}
