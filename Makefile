INTERNAL_BIN_DIR=_internal_bin
GOVERSION=$(shell go version)
GOOS=$(word 1,$(subst /, ,$(lastword $(GOVERSION))))
GOARCH=$(word 2,$(subst /, ,$(lastword $(GOVERSION))))
VERSION=$(patsubst "%",%,$(lastword $(shell grep 'const version' peco.go)))
RELEASE_DIR=releases
ARTIFACTS_DIR=$(RELEASE_DIR)/artifacts/$(VERSION)
SRC_FILES = $(wildcard *.go cmd/peco/*.go internal/*/*.go)
HAVE_GLIDE:=$(shell which glide >/dev/null 2>&1)
GITHUB_USERNAME=peco

.PHONY: clean build build-windows-amd64 build-windows-386 build-linux-amd64 $(RELEASE_DIR)/$(GOOS)/$(GOARCH)/peco$(SUFFIX)

build: $(RELEASE_DIR)/peco_$(GOOS)_$(GOARCH)/peco$(SUFFIX)

$(INTERNAL_BIN_DIR)/$(GOOS)/$(GOARCH)/glide:
ifndef HAVE_GLIDE
	@echo "Installing glide for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(INTERNAL_BIN_DIR)/$(GOOS)/$(GOARCH)
	@wget -q -O - https://github.com/Masterminds/glide/releases/download/0.10.2/glide-0.10.2-$(GOOS)-$(GOARCH).tar.gz | tar xvz
	@mv $(GOOS)-$(GOARCH)/glide $(INTERNAL_BIN_DIR)/$(GOOS)/$(GOARCH)/glide
	@rm -rf $(GOOS)-$(GOARCH)
endif

glide: $(INTERNAL_BIN_DIR)/$(GOOS)/$(GOARCH)/glide

installdeps: glide $(SRC_FILES)
	@echo "Installing dependencies..."
	@PATH=$(INTERNAL_BIN_DIR)/$(GOOS)/$(GOARCH):$(PATH) glide install

build-windows-amd64:
	@$(MAKE) build GOOS=windows GOARCH=amd64 SUFFIX=.exe

build-windows-386:
	@$(MAKE) build GOOS=windows GOARCH=386 SUFFIX=.exe

build-linux-amd64:
	@$(MAKE) build GOOS=linux GOARCH=amd64

build-linux-386:
	@$(MAKE) build GOOS=linux GOARCH=386

build-darwin-amd64:
	@$(MAKE) build GOOS=darwin GOARCH=amd64

build-darwin-386:
	@$(MAKE) build GOOS=darwin GOARCH=386

$(RELEASE_DIR)/peco_$(GOOS)_$(GOARCH)/peco$(SUFFIX):
	go build -o $(RELEASE_DIR)/peco_$(GOOS)_$(GOARCH)/peco$(SUFFIX) cmd/peco/peco.go

all: build-linux-amd64 build-linux-386 build-darwin-amd64 build-darwin-386 build-windows-amd64 build-windows-386

release: release-linux-amd64 release-linux-386 release-darwin-amd64 release-darwin-386 release-windows-amd64 release-windows-386

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

release-linux-386: build-linux-386
	@$(MAKE) release-changes release-readme release-targz GOOS=linux GOARCH=386

release-darwin-amd64: build-darwin-amd64
	@$(MAKE) release-changes release-readme release-zip GOOS=darwin GOARCH=amd64

release-darwin-386: build-darwin-386
	@$(MAKE) release-changes release-readme release-zip GOOS=darwin GOARCH=386

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

release-upload: release-windows-386 release-windows-amd64 release-linux-386 release-linux-amd64 release-darwin-386 release-darwin-amd64 release-github-token
	ghr -u $(GITHUB_USERNAME) -t $(shell cat github_token) --draft --replace $(VERSION) $(ARTIFACTS_DIR)

test: installdeps
	@echo "Running tests..."
	@PATH=$(INTERNAL_BIN_DIR)/$(GOOS)/$(GOARCH):$(PATH) go test -v $(shell glide nv)

clean:
	-rm -rf $(RELEASE_DIR)/*/*
	-rm -rf $(ARTIFACTS_DIR)/*
