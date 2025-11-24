# hugo-link-checker

A fast, comprehensive command-line tool to check links in Hugo-based websites and static sites.

## Features

- **Multi-format support**: Scans Markdown (`.md`) and HTML (`.html`, `.htm`) files
- **Smart link detection**: Finds links in multiple formats:
  - Markdown: `[text](url)`, `<url>`, `[ref]: url`
  - HTML: `<a href="url">`, `<link href="url">`
  - Image links (optional): `![alt](src)`, `<img src="url">`
- **Internal and external link checking**: 
  - Internal links: Validates file existence and Hugo-style URL patterns
  - External links: HTTP/HTTPS status code validation (optional)
- **Hugo-aware**: Understands Hugo content structure and URL patterns
- **Multiple output formats**: Text, JSON, and HTML reports
- **Template syntax handling**: Skips Hugo template syntax like `{{.Site.BaseURL}}`
- **Flexible scanning**: Scan specific directories or entire sites
- **CI/CD friendly**: Exit codes indicate broken link count for automation

## Installation

### Quick start (local build)

```bash
make build
./hugo-link-checker -version
```

### Build cross-platform locally

```bash
make build-all
ls dist
```

## Usage

### Basic usage

```bash
# Check all links in current directory
./hugo-link-checker

# Check specific directory
./hugo-link-checker /path/to/hugo/site

# Check multiple directories
./hugo-link-checker content/ static/ themes/
```

### Command-line flags

| Flag | Description | Default |
|------|-------------|---------|
| `-version` | Print version and exit | `false` |
| `-root <dir>` | Hugo root directory to scan | `.` |
| `-check-external` | Check external HTTP/HTTPS links | `false` |
| `-check-images` | Check image links (img src, markdown images) | `false` |
| `-check-public` | Check for link destinations in Hugo's public directory | `false` |
| `-base-url <url>` | Base URL for checking internal links online (e.g., `https://example.com`) | `""` |
| `-format <format>` | Report format: `text`, `json`, `html` | `text` |
| `-output <file>` | Output file for report (default: stdout) | `""` |
| `-no-report` | Don't generate report, just return exit code | `false` |
| `-verbose` | Show all candidate paths checked for broken internal links | `false` |

### Examples

```bash
# Check only internal links with text output
./hugo-link-checker -root ./content

# Check all links including external ones
./hugo-link-checker -check-external -check-images

# Generate JSON report to file
./hugo-link-checker -format json -output report.json

# Check internal links against live site
./hugo-link-checker -base-url https://mysite.com -check-external

# Check links against Hugo's built public directory
./hugo-link-checker -check-public

# Verbose mode for debugging broken internal links
./hugo-link-checker -verbose

# CI mode: just return exit code (number of broken links)
./hugo-link-checker -no-report -check-external
```

### Exit codes

- `0`: No broken links found
- `1-255`: Number of broken links found (capped at 255)
- `1`: General error (file access, invalid arguments, etc.)

## Output formats

### Text (default)

Human-readable summary with broken links listed by file.

### JSON

Machine-readable format with detailed link information:
```json
{
  "generated_at": "2023-11-21T10:30:00Z",
  "summary": {
    "total_files": 25,
    "total_links": 150,
    "broken_links": 3
  },
  "links": [...]
}
```

### HTML

Web-friendly report.

## GitHub Action

This tool is available as a reusable GitHub Action that can be used in other repositories to check links in Hugo sites and static websites.

### Basic usage

```yaml
name: Check Links
on: [push, pull_request]

jobs:
  link-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: your-username/hugo-link-checker@v1
        with:
          root: './content'
          check-external: true
```

### Action inputs

| Input | Description | Default |
|-------|-------------|---------|
| `root` | Root directory to scan | `.` |
| `check-external` | Check external HTTP/HTTPS links | `false` |
| `check-images` | Check image links | `false` |
| `check-public` | Check for link destinations in Hugo public directory | `false` |
| `base-url` | Base URL for checking internal links online | `""` |
| `format` | Report format: `text`, `json`, `html` | `text` |
| `output` | Output file for report | `""` |
| `verbose` | Show verbose output for debugging | `false` |
| `fail-on-broken-links` | Fail the action if broken links are found | `true` |

### Action outputs

| Output | Description |
|--------|-------------|
| `broken-links-count` | Number of broken links found |
| `report-file` | Path to generated report file (if output specified) |

### Advanced examples

#### Full link checking with report generation

```yaml
name: Comprehensive Link Check
on: [push, pull_request]

jobs:
  link-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Check all links
        uses: your-username/hugo-link-checker@v1
        with:
          root: '.'
          check-external: true
          check-images: true
          format: 'json'
          output: 'link-report.json'
          verbose: true
      
      - name: Upload report
        uses: actions/upload-artifact@v4
        if: always()
        with:
          name: link-check-report
          path: link-report.json
```

#### Check against live site

```yaml
name: Check Links Against Live Site
on: [push, pull_request]

jobs:
  link-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Check internal links against live site
        uses: your-username/hugo-link-checker@v1
        with:
          root: './content'
          base-url: 'https://mysite.com'
          check-external: true
```

#### Non-failing link check for monitoring

```yaml
name: Link Check Monitoring
on:
  schedule:
    - cron: '0 0 * * 0'  # Weekly

jobs:
  link-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Check links (non-failing)
        id: link-check
        uses: your-username/hugo-link-checker@v1
        with:
          check-external: true
          fail-on-broken-links: false
          format: 'html'
          output: 'link-report.html'
      
      - name: Comment on broken links
        if: steps.link-check.outputs.broken-links-count > 0
        run: |
          echo "Found ${{ steps.link-check.outputs.broken-links-count }} broken links"
          # Add notification logic here
```

#### Hugo site with public directory check

```yaml
name: Hugo Site Link Check
on: [push, pull_request]

jobs:
  build-and-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Hugo
        uses: peaceiris/actions-hugo@v2
        with:
          hugo-version: 'latest'
      
      - name: Build Hugo site
        run: hugo --minify
      
      - name: Check links in built site
        uses: your-username/hugo-link-checker@v1
        with:
          root: '.'
          check-public: true
          check-external: true
          base-url: 'https://mysite.com'
```

## Development

This repository contains a Go-based CLI `hugo-link-checker` and CI workflow
to produce compiled binaries for Linux, macOS, and Windows using GitHub Actions.

```bash
# Run tests
make test

# Build for current platform
make build

# Build for all platforms
make build-all

# Run linting
make lint
```
