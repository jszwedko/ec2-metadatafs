export GO15VENDOREXPERIMENT = 1

VERSION := $(shell git describe --tags --always --dirty)
REVISION := $(shell git rev-parse --sq HEAD)
.DEFAULT_GOAL := check

ifndef GOBIN
GOBIN := $(shell echo "$${GOPATH%%:*}/bin")
endif

LINT := $(GOBIN)/golint
GOX := $(GOBIN)/gox
PACKAGES := $$(go list ./... | grep -v '/vendor/')

$(LINT): ; @go get github.com/golang/lint/golint
$(GOX): ; @go get -v github.com/mitchellh/gox

.PHONY: build
build:
	@go build -ldflags "-X main.VersionString=$(VERSION) -X main.RevisionString=$(REVISION)" $(PACKAGES)

.PHONY: install
install:
	@go install -ldflags "-X main.VersionString=$(VERSION) -X main.RevisionString=$(REVISION)" $(PACKAGES)

.PHONY: dist
dist: $(GOX)
	@$(GOX) -ldflags "-X main.VersionString=$(VERSION) -X main.RevisionString=$(REVISION)" -os 'linux' -arch '386 amd64'  -output 'dist/{{.OS}}_{{.Arch}}' .

.PHONY: release
release: dist
	hub release create $$(for f in dist/* ; do echo -n "-a $$f " ; done) $(tag)

.PHONY: vet
vet:
	@go vet $(PACKAGES)

.PHONY: lint
lint: $(LINT)
	@exit $$(for dir in . metadatafs tagsfs ; do $(LINT) $$dir ; done | tee /dev/tty | wc -l)

.PHONY: test
test:
	@go test -cover $(PACKAGES)

.PHONY: check
check: vet lint test build
