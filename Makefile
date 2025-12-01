APP       ?= roo-task-man
TAGS      ?= js_hooks
LDFLAGS   ?= -s -w -X roocode-task-man/internal/version.Version=$(VERSION) -X roocode-task-man/internal/version.Commit=$(COMMIT) -X roocode-task-man/internal/version.Date=$(DATE)
VERSION   ?= $(shell cat VERSION 2>/dev/null || git describe --tags --always --dirty 2>/dev/null || echo 0.1.0)
COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE      ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
DIST      ?= dist

.PHONY: build run test tidy clean release release-darwin-arm64 release-darwin-amd64 release-windows-amd64

build:
	go build -tags $(TAGS) -ldflags "$(LDFLAGS)" -o $(APP) ./cmd/roo-task-man

run: build
	./$(APP)

test:
	go test ./...

tidy:
	go mod tidy

clean:
	rm -rf $(APP) $(DIST)

# -------- Release (cross-compile) --------
$(DIST):
	mkdir -p $(DIST)

release: $(DIST) release-darwin-arm64 release-darwin-amd64 release-windows-amd64
	@echo "Release artifacts in $(DIST)"

release-darwin-arm64: $(DIST)
	@echo "Building macOS arm64…"
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 \
		go build -tags $(TAGS) -ldflags "$(LDFLAGS)" -o $(DIST)/$(APP) ./cmd/roo-task-man
	mkdir -p $(DIST)/$(APP)_$(VERSION)_darwin_arm64 && \
	cp $(DIST)/$(APP) $(DIST)/$(APP)_$(VERSION)_darwin_arm64/ && \
	tar -C $(DIST)/$(APP)_$(VERSION)_darwin_arm64 -czf $(DIST)/$(APP)_$(VERSION)_darwin_arm64.tar.gz . && \
	rm -rf $(DIST)/$(APP)_$(VERSION)_darwin_arm64 $(DIST)/$(APP)

release-darwin-amd64: $(DIST)
	@echo "Building macOS amd64…"
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 \
		go build -tags $(TAGS) -ldflags "$(LDFLAGS)" -o $(DIST)/$(APP) ./cmd/roo-task-man
	mkdir -p $(DIST)/$(APP)_$(VERSION)_darwin_amd64 && \
	cp $(DIST)/$(APP) $(DIST)/$(APP)_$(VERSION)_darwin_amd64/ && \
	tar -C $(DIST)/$(APP)_$(VERSION)_darwin_amd64 -czf $(DIST)/$(APP)_$(VERSION)_darwin_amd64.tar.gz . && \
	rm -rf $(DIST)/$(APP)_$(VERSION)_darwin_amd64 $(DIST)/$(APP)

release-windows-amd64: $(DIST)
	@echo "Building Windows amd64…"
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
	go build -tags $(TAGS) -ldflags "$(LDFLAGS)" -o $(DIST)/$(APP).exe ./cmd/roo-task-man
	mkdir -p $(DIST)/$(APP)_$(VERSION)_windows_amd64 && \
	mv $(DIST)/$(APP).exe $(DIST)/$(APP)_$(VERSION)_windows_amd64/ && \
	cd $(DIST) && zip -rq $(APP)_$(VERSION)_windows_amd64.zip $(APP)_$(VERSION)_windows_amd64 && \
	rm -rf $(DIST)/$(APP)_$(VERSION)_windows_amd64

.PHONY: version-update
version-update:
	@echo "Updating VERSION from git describe…"
	@v=$$(git describe --tags --always --dirty 2>/dev/null || echo $(VERSION)); echo $$v > VERSION; echo "VERSION=$$v"
