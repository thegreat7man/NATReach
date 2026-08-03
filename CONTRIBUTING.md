# Contributing to NATReach

Contributions are welcome. Keep changes portable, dependency-light, and easy for non-technical users to run.

## Development setup

1. Install Go 1.25 or newer.
2. Fork and clone the repository.
3. Create a focused branch.
4. Run the local checks:

```bash
go test ./...
go vet ./...
```

The public-tunnel integration test is opt-in because it contacts an external service:

```bash
NATREACH_INTEGRATION=1 go test -run TestEndToEndSSHIntegration -v
```

## Pull requests

- Explain the user-visible behavior and security impact.
- Keep platform-specific code behind Go build constraints.
- Avoid background services, global installers, and unnecessary runtime dependencies.
- Update the README and tests when behavior changes.
- Never commit credentials, generated release archives, or local binaries.
