# hugo-link-checker

CLI tool for validating links in Hugo static sites. Checks internal and external links in Markdown and HTML files.

## Build

```bash
go build -o hugo-link-checker ./cmd/hugo-link-checker

# Or via Task
task build
```

## Test

```bash
go test -race -count=1 ./...

# Or via Task
task test
```

## Lint

```bash
# Requires golangci-lint v2
golangci-lint run ./...

# Or via Task
task lint
```

## Vulnerability Check

```bash
govulncheck ./...

# Or via Task
task vulncheck
```

## All CI Checks

```bash
task check
```

## Key Packages

- `internal/checker` — link validation logic
- `internal/scanner` — file scanning and link extraction
- `internal/reporter` — output formatting (text, JSON, HTML)
- `internal/version` — version string
