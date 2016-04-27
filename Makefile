INTERNAL_BIN_DIR=_internal_bin
GOVERSION=$(shell go version)
GOOS=$(word 1,$(subst /, ,$(lastword $(GOVERSION))))
GOARCH=$(word 2,$(subst /, ,$(lastword $(GOVERSION))))
RELEASE_DIR=releases
SRC_FILES = $(wildcard *.go internal/*/*.go)

.PHONY: build build-windows-amd64 build-windows-386 build-linux-amd64 $(RELEASE_DIR)/$(GOOS)/$(GOARCH)/peco$(SUFFIX)

$(INTERNAL_BIN_DIR)/$(GOOS)/$(GOARCH)/glide:
	@echo "Installing glide for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(INTERNAL_BIN_DIR)/$(GOOS)/$(GOARCH)
	@wget -q -O - https://github.com/Masterminds/glide/releases/download/0.10.2/glide-0.10.2-$(GOOS)-$(GOARCH).tar.gz | tar xvz
	@mv $(GOOS)-$(GOARCH)/glide $(INTERNAL_BIN_DIR)/$(GOOS)/$(GOARCH)/glide
	@rm -rf $(GOOS)-$(GOARCH)

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

$(RELEASE_DIR)/$(GOOS)/$(GOARCH)/peco$(SUFFIX): $(SRC_FILES)
	go build -o $(RELEASE_DIR)/$(GOOS)/$(GOARCH)/peco$(SUFFIX)

build: $(RELEASE_DIR)/$(GOOS)/$(GOARCH)/peco$(SUFFIX)

all:
	@$(MAKE) build-windows-amd64 
	@$(MAKE) build-windows-386
	@$(MAKE) build-linux-amd64
	@$(MAKE) build-linux-386


test: installdeps
	@echo "Running tests..."
	@PATH=$(INTERNAL_BIN_DIR)/$(GOOS)/$(GOARCH):$(PATH) go test -v $(shell glide nv)
