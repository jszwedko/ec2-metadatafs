export GO15VENDOREXPERIMENT = 1

VERSION = 1.0.0

PREFIX ?= /usr/local
MANPREFIX ?= $(PREFIX)/share/man
INSTALL ?= install

BIN = ec2-metadatafs
PACKAGES = $$(go list ./... | grep -v '/vendor/')
BINPACKAGE := github.com/jszwedko/ec2-metadatafs

.DEFAULT_GOAL := check

ifndef GOBIN
GOBIN := $(shell echo "$${GOPATH%%:*}/bin")
endif

LINT := $(GOBIN)/golint
GOX := $(GOBIN)/gox

$(LINT): ; @go get golang.org/x/lint/golint
$(GOX): ; @go get -v github.com/mitchellh/gox

.PHONY: build
build:
	@go build -ldflags "-X main.VersionString=$(VERSION)" -o $(BIN) $(BINPACKAGE)

.PHONY: install
install: build
	$(INSTALL) -m 0755 -d $(DESTDIR)$(PREFIX)/bin
	$(INSTALL) -m 0755 $(BIN) $(DESTDIR)$(PREFIX)/bin
	$(INSTALL) -m 0755 -d $(DESTDIR)$(MANPREFIX)/man1
	$(INSTALL) -m 0644 $(BIN).1 $(DESTDIR)$(MANPREFIX)/man1

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
