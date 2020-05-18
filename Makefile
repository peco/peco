INTERNAL_BIN_DIR=_internal_bin
GOVERSION=$(shell go version)
THIS_GOOS=$(word 1,$(subst /, ,$(lastword $(GOVERSION))))
THIS_GOARCH=$(word 2,$(subst /, ,$(lastword $(GOVERSION))))
GOOS=$(THIS_GOOS)
GOARCH=$(THIS_GOARCH)
VERSION=$(patsubst "%",%,$(lastword $(shell grep 'const version' peco.go)))
RELEASE_DIR=releases
ARTIFACTS_DIR=$(RELEASE_DIR)/artifacts/$(VERSION)
SRC_FILES = $(wildcard *.go cmd/peco/*.go internal/*/*.go)
GITHUB_USERNAME=peco
BUILD_TARGETS= \
	build-linux-arm64 \
	build-linux-arm \
	build-linux-amd64 \
	build-linux-386 \
	build-darwin-amd64 \
	build-windows-amd64 \
	build-windows-386
RELEASE_TARGETS=\
	release-linux-arm64 \
	release-linux-arm \
	release-linux-amd64 \
	release-linux-386 \
	release-darwin-amd64 \
	release-windows-amd64 \
	release-windows-386

.PHONY: clean build $(RELEASE_TARGETS) $(BUILD_TARGETS) $(RELEASE_DIR)/$(GOOS)/$(GOARCH)/peco$(SUFFIX)

build: $(RELEASE_DIR)/peco_$(GOOS)_$(GOARCH)/peco$(SUFFIX)

$(INTERNAL_BIN_DIR):
	@echo "Creating $(INTERNAL_BIN_DIR)"
	@mkdir -p $(INTERNAL_BIN_DIR)

build-windows-amd64:
	@$(MAKE) build GOOS=windows GOARCH=amd64 SUFFIX=.exe

build-windows-386:
	@$(MAKE) build GOOS=windows GOARCH=386 SUFFIX=.exe

build-linux-amd64:
	@$(MAKE) build GOOS=linux GOARCH=amd64

build-linux-arm:
	@$(MAKE) build GOOS=linux GOARCH=arm

build-linux-arm64:
	@$(MAKE) build GOOS=linux GOARCH=arm64

build-linux-386:
	@$(MAKE) build GOOS=linux GOARCH=386

build-darwin-amd64:
	@$(MAKE) build GOOS=darwin GOARCH=amd64

$(RELEASE_DIR)/peco_$(GOOS)_$(GOARCH)/peco$(SUFFIX):
	@GO111MODULE=on go build -o $(RELEASE_DIR)/peco_$(GOOS)_$(GOARCH)/peco$(SUFFIX) cmd/peco/peco.go

all: $(BUILD_TARGETS)

release: $(RELEASE_TARGETS)

$(RELEASE_DIR)/peco_$(GOOS)_$(GOARCH)/Changes:
	@cp Changes $(RELEASE_DIR)/peco_$(GOOS)_$(GOARCH)

$(RELEASE_DIR)/peco_$(GOOS)_$(GOARCH)/README.md:
	@cp README.md $(RELEASE_DIR)/peco_$(GOOS)_$(GOARCH)

release-changes: $(RELEASE_DIR)/peco_$(GOOS)_$(GOARCH)/Changes
release-readme: $(RELEASE_DIR)/peco_$(GOOS)_$(GOARCH)/README.md

release-windows-amd64: build-windows-amd64
	@$(MAKE) release-changes release-readme release-zip GOOS=windows GOARCH=amd64

release-windows-386: build-windows-386
	@$(MAKE) release-changes release-readme release-zip GOOS=windows GOARCH=386

release-linux-amd64: build-linux-amd64
	@$(MAKE) release-changes release-readme release-targz GOOS=linux GOARCH=amd64

release-linux-arm: build-linux-arm
	@$(MAKE) release-changes release-readme release-targz GOOS=linux GOARCH=arm

release-linux-arm64: build-linux-arm64
	@$(MAKE) release-changes release-readme release-targz GOOS=linux GOARCH=arm64

release-linux-386: build-linux-386
	@$(MAKE) release-changes release-readme release-targz GOOS=linux GOARCH=386

release-darwin-amd64: build-darwin-amd64
	@$(MAKE) release-changes release-readme release-zip GOOS=darwin GOARCH=amd64

$(ARTIFACTS_DIR):
	@mkdir -p $(ARTIFACTS_DIR)

# note: I dreamt of using tar.bz2 for my releases, but then historically
# (for whatever reason that is unknwon to me now) I was creating .zip for
# darwin/windows, and .tar.gz for linux, so I guess we'll stick with those.
# (I think this is from goxc days)
release-tarbz: $(ARTIFACTS_DIR)
	tar -cjf $(ARTIFACTS_DIR)/peco_$(GOOS)_$(GOARCH).tar.bz2 -C $(RELEASE_DIR) peco_$(GOOS)_$(GOARCH)

release-targz: $(ARTIFACTS_DIR)
	tar -czf $(ARTIFACTS_DIR)/peco_$(GOOS)_$(GOARCH).tar.gz -C $(RELEASE_DIR) peco_$(GOOS)_$(GOARCH)

release-zip: $(ARTIFACTS_DIR)
	cd $(RELEASE_DIR) && zip -9 $(CURDIR)/$(ARTIFACTS_DIR)/peco_$(GOOS)_$(GOARCH).zip peco_$(GOOS)_$(GOARCH)/*

release-github-token: github_token
	@echo "file `github_token` is required"

release-upload: release release-github-token
	ghr -u $(GITHUB_USERNAME) -t $(shell cat github_token) --draft --replace $(VERSION) $(ARTIFACTS_DIR)

test:
	@echo "Running tests..."
	@GO111MODULE=on PATH=$(INTERNAL_BIN_DIR)/$(GOOS)/$(GOARCH):$(PATH) go test -v ./...

clean:
	-rm -rf $(RELEASE_DIR)/*/*
	-rm -rf $(ARTIFACTS_DIR)/*

lint:
	golangci-lint run ./...

