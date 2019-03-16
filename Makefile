# Adapted from https://github.com/alibaba/terraform-provider/blob/master/Makefile

VETARGS?=-all
TEST?=$$(go list ./...)
ifeq ($(OS),Windows_NT)
	UNAME := Windows
else
	UNAME := $(shell uname -s)
endif
os := noos
ifeq ($(UNAME),Windows)
	os := windows
endif
ifeq ($(UNAME),Darwin)
	os := mac
endif
ifeq ($(UNAME),Linux)
	os := linux
endif

TAG := ${TRAVIS_TAG}
ifndef TAG
	TAG := $(shell git describe --abbrev=0)
endif

HASZIP := $(shell command -v zip 2> /dev/null)

all: build

build: mac windows linux

install: $(os) copy-$(os)

dev: clean fmt install
	go mod tidy

copy-mac:
	rm -f bin/terraform-provider-zerotier
	mkdir -p "${HOME}/.terraform.d/plugins/darwin_amd64"
	tar -xvf bin/terraform-provider-zerotier_darwin-amd64_$(TAG).tgz && \
		mv bin/terraform-provider-zerotier_$(TAG) "${HOME}/.terraform.d/plugins/darwin_amd64/"

copy-linux:
	rm -f bin/terraform-provider-zerotier
	mkdir -p "${HOME}/.terraform.d/plugins/linux_amd64"
	tar -xvf bin/terraform-provider-zerotier_darwin-amd64_$(TAG).tgz && \
		mv bin/terraform-provider-zerotier_$(TAG) "${HOME}/.terraform.d/plugins/linux_amd64/"

copy-windows:
	rm -f bin/terraform-provider-zerotier
	mkdir -p "${APPDATA}/terraform.d/plugins/windows_amd64"
	unzip bin/terraform-provider-zerotier_windows-amd64_$(TAG).zip && \
		mv bin/terraform-provider-zerotier_$(TAG).exe "${APPDATA}/terraform.d/plugins/windows_amd64/"

test: vet fmtcheck errcheck
	TF_ACC=1 go test -v -run=TestAcczerotier -timeout=180m -parallel=4

vet:
	@echo "go vet $(VETARGS) ."
	@go vet $(VETARGS); if [ $$? -eq 1 ]; then \
		echo ""; \
		echo "Vet found suspicious constructs. Please check the reported constructs"; \
		echo "and fix them if necessary before submitting the code for review."; \
		exit 1; \
	fi

fmt:
	gofmt -w .
	goimports -w .

fmtcheck:
	@sh -c "'$(CURDIR)/scripts/gofmtcheck.sh'"

errcheck:
	@sh -c "'$(CURDIR)/scripts/errcheck.sh'"

clean:
	rm -rf bin/*

mac:
	GOOS=darwin GOARCH=amd64 go build -o bin/terraform-provider-zerotier_$(TAG)
	tar czvf bin/terraform-provider-zerotier_darwin-amd64_$(TAG).tgz bin/terraform-provider-zerotier_$(TAG)
	rm -rf bin/terraform-provider-zerotier_$(TAG)

windows:
ifndef HASZIP
	$(error "zip is not available. If you're on windows, try `choco install zip`")
endif
	GOOS=windows GOARCH=amd64 go build -o bin/terraform-provider-zerotier_$(TAG).exe
	zip bin/terraform-provider-zerotier_windows-amd64_$(TAG).zip bin/terraform-provider-zerotier_$(TAG).exe
	rm -rf bin/terraform-provider-zerotier_$(TAG).exe

linux:
	GOOS=linux GOARCH=amd64 go build -o bin/terraform-provider-zerotier_$(TAG)
	tar czvf bin/terraform-provider-zerotier_linux-amd64_$(TAG).tgz bin/terraform-provider-zerotier_$(TAG)
	rm -rf bin/terraform-provider-zerotier_$(TAG)

noos:
	@echo "your OS was not detected"

copy-noos:
