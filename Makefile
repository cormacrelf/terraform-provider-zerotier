# Adapted from https://github.com/alibaba/terraform-provider/blob/master/Makefile

GOFMT_FILES?=$$(find . -name '*.go' | grep -v vendor)
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

TAG := $(shell git describe --abbrev=0)

all: build

build: mac windows linux

install: $(os) copy-$(os)

dev: clean fmt install

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
	TF_ACC=1 go test -v ./zerotier -run=TestAcczerotier -timeout=180m -parallel=4

vet:
	@echo "go tool vet $(VETARGS) ."
	@go tool vet $(VETARGS) $$(ls -d */ | grep -v vendor) ; if [ $$? -eq 1 ]; then \
		echo ""; \
		echo "Vet found suspicious constructs. Please check the reported constructs"; \
		echo "and fix them if necessary before submitting the code for review."; \
		exit 1; \
	fi

fmt:
	gofmt -w $(GOFMT_FILES)
	goimports -w $(GOFMT_FILES)

fmtcheck:
	@sh -c "'$(CURDIR)/scripts/gofmtcheck.sh'"

errcheck:
	@sh -c "'$(CURDIR)/scripts/errcheck.sh'"

clean:
	rm -rf bin/*

mac:
	GOOS=darwin GOARCH=amd64 go build -o bin/terraform-provider-zerotier_$(TAG) ./zerotier
	tar czvf bin/terraform-provider-zerotier_darwin-amd64_$(TAG).tgz bin/terraform-provider-zerotier_$(TAG)
	rm -rf bin/terraform-provider-zerotier_$(TAG)

windows:
	GOOS=windows GOARCH=amd64 go build -o bin/terraform-provider-zerotier_$(TAG).exe ./zerotier
	zip bin/terraform-provider-zerotier_windows-amd64_$(TAG).zip bin/terraform-provider-zerotier_$(TAG).exe
	rm -rf bin/terraform-provider-zerotier_$(TAG).exe

linux:
	GOOS=linux GOARCH=amd64 go build -o bin/terraform-provider-zerotier_$(TAG) ./zerotier
	tar czvf bin/terraform-provider-zerotier_linux-amd64_$(TAG).tgz bin/terraform-provider-zerotier_$(TAG)
	rm -rf bin/terraform-provider-zerotier_$(TAG)

noos:
	@echo "your OS was not detected"

copy-noos:
