BINARY   := gitflow
MODULE   := github.com/luis-lozano/gitflow-helper
VERSION  ?= $(shell cat VERSION 2>/dev/null || git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -s -w -X main.version=$(VERSION)
BUILD    := CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)'
DIST     := dist

.PHONY: build build-all universal clean test vet lint release install

## build: compile for current platform
build:
	$(BUILD) -o $(BINARY) ./cmd/gitflow

## build-all: cross-compile all targets
build-all: clean
	@mkdir -p $(DIST)
	@echo "→ Linux amd64"
	GOOS=linux   GOARCH=amd64 $(BUILD) -o $(DIST)/$(BINARY)-linux-amd64       ./cmd/gitflow
	@echo "→ Windows amd64"
	GOOS=windows GOARCH=amd64 $(BUILD) -o $(DIST)/$(BINARY)-windows-amd64.exe ./cmd/gitflow
	@echo "→ macOS amd64"
	GOOS=darwin  GOARCH=amd64 $(BUILD) -o $(DIST)/$(BINARY)-darwin-amd64      ./cmd/gitflow
	@echo "→ macOS arm64"
	GOOS=darwin  GOARCH=arm64 $(BUILD) -o $(DIST)/$(BINARY)-darwin-arm64      ./cmd/gitflow
	@echo "Done. Binaries in $(DIST)/"

## universal: create macOS universal binary (requires lipo)
universal: build-all
	@echo "→ macOS universal binary"
	lipo -create -output $(DIST)/$(BINARY)-darwin-universal \
		$(DIST)/$(BINARY)-darwin-amd64 \
		$(DIST)/$(BINARY)-darwin-arm64
	@echo "Created $(DIST)/$(BINARY)-darwin-universal"

## clean: remove build artifacts
clean:
	rm -rf $(DIST) $(BINARY)

## test: run all tests
test:
	go test ./... -v

## vet: run go vet
vet:
	go vet ./...

## lint: run staticcheck (install with go install honnef.co/go/tools/cmd/staticcheck@latest)
lint:
	@command -v staticcheck >/dev/null 2>&1 || { echo "install staticcheck first"; exit 1; }
	staticcheck ./...

## release: run goreleaser (requires goreleaser installed)
release:
	goreleaser release --clean

## release-snapshot: test goreleaser locally without publishing
release-snapshot:
	goreleaser release --snapshot --clean

## install: install to GOPATH/bin
install:
	go install -ldflags '$(LDFLAGS)' ./cmd/gitflow

## version: print the current version
version:
	@echo $(VERSION)
