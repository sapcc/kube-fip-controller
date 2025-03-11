DATE    = $(shell date +%Y%m%d%H%M)
IMAGE   ?= ghcr.io/sapcc/kube-fip-controller
VERSION = v$(DATE)
GOOS    ?= $(shell go env GOOS)
BINARY  := controller
OPTS    ?=

SRCDIRS  := cmd pkg
PACKAGES := $(shell find $(SRCDIRS) -type d)
GO_PKG	 := github.com/sapcc/kube-fip-controller
GOFILES  := $(addsuffix /*.go,$(PACKAGES))
GOFILES  := $(wildcard $(GOFILES))

.PHONY: all clean vendor tests static-check

all: bin/$(GOOS)/$(BINARY)

bin/%/$(BINARY): GIT_COMMIT  = $(shell git rev-parse --short HEAD)
bin/%/$(BINARY): BUILD_DATE  = $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
bin/%/$(BINARY): $(GOFILES) Makefile
	GOOS=$* GOARCH=amd64 go build -ldflags '-X github.com/sapcc/kube-fip-controller/cmd.BuildCommit=$(GIT_COMMIT) -X github.com/sapcc/kube-fip-controller/cmd.BuildDate=$(BUILD_DATE)' -mod vendor -v -o bin/$*/$(BINARY) ./cmd/main.go && chmod +x bin/$*/$(BINARY)

build:
	docker build $(OPTS) -t $(IMAGE):$(VERSION) .

static-check:
	@if s="$$(gofmt -s -l *.go pkg 2>/dev/null)"                            && test -n "$$s"; then printf ' => %s\n%s\n' gofmt  "$$s"; false; fi
	@if s="$$(golint . && find pkg -type d -exec golint {} \; 2>/dev/null)" && test -n "$$s"; then printf ' => %s\n%s\n' golint "$$s"; false; fi

tests: all static-check
	DEBUG=1 && go test -v github.com/sapcc/kube-fip-controller/pkg/controller

push: build
	docker push $(IMAGE):$(VERSION)

clean:
	rm -rf bin/*

vendor:
	go mod tidy && go mod vendor

install-go-licence-detector: FORCE
	@if ! hash go-licence-detector 2>/dev/null; then printf "\e[1;36m>> Installing go-licence-detector (this may take a while)...\e[0m\n"; go install go.elastic.co/go-licence-detector@latest; fi

check-dependency-licenses: FORCE install-go-licence-detector
	@printf "\e[1;36m>> go-licence-detector\e[0m\n"
	@go list -m -mod=readonly -json all | go-licence-detector -includeIndirect -rules .license-scan-rules.json -overrides .license-scan-overrides.jsonl
