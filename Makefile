.PHONY: build build-all clean deb-local
BINARY := hugo-link-checker
DIST := dist
VERSION := $(shell grep 'var Version' internal/version/version.go | cut -d'"' -f2)

build:
	go build -o $(BINARY) ./cmd/hugo-link-checker

build-all:
	@echo "Building for multiple OS/ARCH..."
	@rm -rf $(DIST) && mkdir -p $(DIST)
	@for os_arch in "linux/amd64" "linux/arm64" "darwin/amd64" "darwin/arm64" "windows/amd64"; do \
	  os=$${os_arch%/*}; arch=$${os_arch#*/}; \
	  ext=""; if [ "$${os}" = "windows" ]; then ext=".exe"; fi; \
	  echo "Building $$os/$$arch"; \
	  CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -o $(DIST)/$(BINARY)-$$os-$$arch$$ext ./cmd/hugo-link-checker; \
	done

clean:
	rm -f $(BINARY)
	rm -rf $(DIST)

deb-local:
	@echo "Building Debian package locally using debian/ control files..."
	@echo "Checking for required tools..."
	@which dpkg-buildpackage > /dev/null || (echo "Error: dpkg-buildpackage not found. Install with: sudo apt-get install dpkg-dev" && exit 1)
	@which dh > /dev/null || (echo "Error: debhelper not found. Install with: sudo apt-get install debhelper" && exit 1)
	@which go > /dev/null || (echo "Error: go not found. Install golang" && exit 1)
	@dpkg -l dh-golang > /dev/null 2>&1 || (echo "Error: dh-golang not found. Install with: sudo apt-get install dh-golang" && exit 1)
	@echo "Building package with dpkg-buildpackage..."
	dpkg-buildpackage -us -uc -b
	@echo "Debian package built successfully!"
	@echo "Package files created in parent directory:"
	@ls -la ../hugo-link-checker_*.deb ../hugo-link-checker_*.buildinfo ../hugo-link-checker_*.changes 2>/dev/null || true
