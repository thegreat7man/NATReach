# Security Policy

NATReach exposes an interactive SSH shell and should be treated as security-sensitive software.

## Reporting a vulnerability

Do not open a public issue for a suspected vulnerability.

Use [GitHub private vulnerability reporting](https://github.com/thegreat7man/NATReach/security/advisories/new) and include:

- the affected NATReach version and operating system;
- clear reproduction steps;
- the expected and observed behavior;
- any suggested mitigation, if available.

Please allow a reasonable amount of time for investigation before public disclosure.

## Supported versions

Only the latest published release receives security fixes.

## Operational guidance

- Compare the SSH host-key fingerprint before accepting a new connection.
- Stop NATReach as soon as remote access is no longer required.
- Prefer Tailscale, a private VPN, or a self-hosted relay for persistent sensitive access.
- Do not weaken the host operating system's sudo, UAC, firewall, or privacy controls to simplify setup.
