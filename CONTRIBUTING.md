# Contributing

## Development

```bash
git clone https://github.com/dakaneye/kora.git
cd kora
make build
```

## Prerequisites

- Go 1.25+
- [`gh`](https://cli.github.com/) (GitHub CLI)
- [`gws`](https://github.com/googleworkspace/cli) (Google Workspace CLI)
- [`linear`](https://github.com/schpet/linear-cli) (Linear CLI)

## Commands

```bash
make build            # Compile
make test             # Unit tests
make test-integration # Integration tests (requires auth)
make test-e2e         # E2E tests (builds binary)
make lint             # golangci-lint
```

## Before Submitting

1. `make lint` passes
2. `make test` passes
3. New functionality has tests
4. Commit messages follow conventional commits (`feat:`, `fix:`, `docs:`, etc.)

## Pull Requests

- Keep changes focused
- Update tests for new functionality
- Follow existing code style
- One source per PR when adding new data sources
