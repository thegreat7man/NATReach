# NATReach

[![CI](https://github.com/thegreat7man/NATReach/actions/workflows/ci.yml/badge.svg)](https://github.com/thegreat7man/NATReach/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/thegreat7man/NATReach)](https://github.com/thegreat7man/NATReach/releases/latest)
[![License](https://img.shields.io/github/license/thegreat7man/NATReach)](LICENSE)

**Portable SSH access to computers behind NAT.**

NATReach is a single executable with an interactive terminal menu: no browser, WebView, Docker, Python, system `sshd`, background service, or installation process.

The application includes:

- an embedded SSH server with real PTY/ConPTY support;
- SFTP file transfer;
- a free Pinggy TCP tunnel that requires no account;
- a new 192-bit one-time password and SSH host key for every run;
- automatic tunnel reconnection;
- `sudo -i` on Linux/macOS and a UAC prompt on Windows;
- immediate shutdown of the tunnel and active SSH sessions when stopped.

## Quick start

### Windows

Extract the archive and double-click `natreach.exe`.

The unsigned development build may trigger a SmartScreen warning. A commercial code-signing certificate has not been added yet.

### macOS

Extract the archive and double-click `NATReach.command`. It opens the bundled `natreach` executable in Terminal.

The builds are not yet notarized with a paid Apple Developer certificate. If Gatekeeper warns about an unidentified developer, right-click `NATReach.command` in Finder and select **Open** once.

### Linux

```bash
chmod +x natreach
./natreach
```

NATReach displays a simple menu:

```text
SSH is off.
  1 - start regular SSH
  2 - start SSH with a sudo/root shell
  q - exit
```

After startup, NATReach prints a ready-to-use SSH command, one-time password, host-key fingerprint, and SFTP command. While it is running, use `i` for connection details, `l` for logs, `s` to stop, or `q` to stop and exit.

Closing the process window also closes the local server and network connection at the operating-system level. NATReach never installs or leaves a background service behind.

## Tunnel provider

There is no single best free tunnel provider for every use case:

| Provider | Main advantage | Trade-off for NATReach |
|---|---|---|
| Tailscale Personal | Best fit for persistent personal access, stable names, and a private network | Requires an account and Tailscale on both devices |
| Cloudflare Tunnel | Reliable infrastructure and strong access controls | Raw TCP/SSH requires Cloudflare client tooling on the connecting side plus account/domain configuration |
| ngrok Free | Established service without a 60-minute TCP timeout | Free TCP requires an account and payment-card verification and has a traffic limit |
| localhost.run | Account-free HTTP/TLS tunnels | Public raw SSH/TCP is not its primary free use case |
| Pinggy Free | A standard public TCP endpoint with no account or second agent | Temporary endpoint and a 60-minute session limit |

Pinggy is therefore the default for the specific requirement of one portable file producing a standard SSH command usable from another computer. For persistent private access, Tailscale is a better choice when installing a client on both devices is acceptable.

NATReach reconnects automatically after a free Pinggy session ends and prints the new connection command. A permanent endpoint is not available on the free plan.

## Administrator mode

- **Linux / macOS:** menu option `2` starts `sudo -i` after SSH login. The operating-system password is entered directly inside the encrypted SSH session. NATReach does not read, intercept, or store it.
- **Windows:** menu option `2` displays the standard UAC prompt and relaunches NATReach with Administrator privileges.

Running the entire Linux/macOS tunnel process as root is intentionally unnecessary. Only the remote shell requests elevation, which keeps the network-facing application at normal user privilege until administrative access is actually needed.

On macOS, access to Desktop, Documents, and other protected folders is also controlled by macOS privacy permissions. A user at the host computer may need to approve the operating-system prompt; NATReach cannot and should not bypass it.

## Automatic mode

Start SSH immediately and wait for `Ctrl+C`:

```bash
./natreach --start
```

Start immediately with an administrator shell:

```bash
./natreach --start --admin
```

The legacy `--cli` flag remains an alias for `--start`.

## Compatibility

- Windows 10 / Server 2016 or newer, AMD64 and ARM64;
- Linux kernel 3.2 or newer, AMD64 and ARM64;
- macOS 12 Monterey or newer, standard AMD64 and ARM64 builds;
- macOS 11 Big Sur, separate archives containing `darwin-bigsur` in the filename.

Shipping a network-facing administrator shell for systems older than Big Sur would be unsafe because Apple and currently supported Go toolchains no longer provide security fixes for them. Intel Macs are supported when they run Big Sur or newer.

Windows versions older than Windows 10 do not provide the required ConPTY support and are no longer supported by current Go toolchains.

## Connecting

From a regular terminal:

```bash
ssh -p PUBLIC_PORT remote@PUBLIC_HOST
```

Before accepting the first connection, compare the host-key fingerprint shown by the SSH client with the fingerprint printed by NATReach.

For SFTP:

```bash
sftp -P PUBLIC_PORT remote@PUBLIC_HOST
```

## Security model

- Only the embedded SSH server on a random loopback port is published.
- The password and host key exist only in memory for the lifetime of the process.
- Pinggy forwards an already encrypted SSH stream and cannot read terminal commands or SFTP contents.
- Incorrect password attempts are deliberately rate-limited.
- There is no browser-based control panel or local HTTP management port.
- For persistent or especially sensitive systems, use Tailscale, a private VPN/relay, and permanent SSH keys.

## Building

End users do not need to compile anything. Ready-to-run archives are provided in `dist` and can be attached to GitHub Releases.

Standard builds require Go 1.25 or newer:

```bash
bash ./scripts/build-all.sh
```

Big Sur-compatible archives are built with Go 1.24.13:

```bash
bash ./scripts/build-legacy-mac.sh
```

On Windows, use `scripts/build-all.ps1`. Every binary is built with `CGO_ENABLED=0` and requires no external runtime libraries.

## Verification

```bash
go test ./...
go vet ./...
NATREACH_INTEGRATION=1 go test -run Integration -v
```

The integration test creates a real public tunnel, signs in over SSH, runs a remote command, and verifies SFTP access.

Release archive checksums are stored in `dist/SHA256SUMS`.

## License

MIT. See `LICENSE` and `go.mod`.
