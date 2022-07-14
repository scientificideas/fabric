# Copyright IBM Corp All Rights Reserved.
# Copyright London Stock Exchange Group All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0

GOTOOLS = counterfeiter golint goimports protoc-gen-go ginkgo gocov gocov-xml misspell
BUILD_DIR ?= .build
GOTOOLS_GOPATH ?= $(BUILD_DIR)/gotools
GOTOOLS_BINDIR ?= $(GOPATH)/bin

# go tool->path mapping
go.fqp.counterfeiter := github.com/maxbrunsfeld/counterfeiter/v6@v6.2.0
go.fqp.gocov         := github.com/axw/gocov/gocov@v1.0.0
go.fqp.gocov-xml     := github.com/AlekSi/gocov-xml@v1.0.0
go.fqp.goimports     := golang.org/x/tools/cmd/goimports
go.fqp.golint        := golang.org/x/lint/golint
go.fqp.manifest-tool := github.com/estesp/manifest-tool
go.fqp.misspell      := github.com/client9/misspell/cmd/misspell
go.fqp.protoc-gen-go := github.com/golang/protobuf/protoc-gen-go
go.fqp.ginkgo        := github.com/onsi/ginkgo/ginkgo

.PHONY: gotools-install
gotools-install: $(patsubst %,$(GOTOOLS_BINDIR)/%, $(GOTOOLS))

.PHONY: gotools-clean
gotools-clean:
	-@rm -rf $(BUILD_DIR)/gotools

# Special override for protoc-gen-go since we want to use the version vendored with the project
gotool.protoc-gen-go:
	@echo "Building github.com/golang/protobuf/protoc-gen-go -> protoc-gen-go"
	GOBIN=$(abspath $(GOTOOLS_BINDIR)) go install ${go.fqp.protoc-gen-go}

# Special override for ginkgo since we want to use the version vendored with the project
gotool.ginkgo:
	@echo "Building github.com/onsi/ginkgo/ginkgo -> ginkgo"
	GOBIN=$(abspath $(GOTOOLS_BINDIR)) go install ${go.fqp.ginkgo}

# Special override for goimports since we want to use the version vendored with the project
gotool.goimports:
	@echo "Building golang.org/x/tools/cmd/goimports -> goimports"
	GOBIN=$(abspath $(GOTOOLS_BINDIR)) go install ${go.fqp.goimports}

# Special override for golint since we want to use the version vendored with the project
gotool.golint:
	@echo "Building golang.org/x/lint/golint -> golint"
	GOBIN=$(abspath $(GOTOOLS_BINDIR)) go install ${go.fqp.golint}

# Special override for golint since we want to use the version vendored with the project
gotool.counterfeiter:
	@echo "Building github.com/maxbrunsfeld/counterfeiter/v6 -> counterfeiter"
	GOBIN=$(abspath $(GOTOOLS_BINDIR)) go install ${go.fqp.counterfeiter}

# Default rule for gotools uses the name->path map for a generic 'go get' style build
gotool.misspell:
	@echo "Building github.com/client9/misspell/cmd/misspell -> misspell"
	GOBIN=$(abspath $(GOTOOLS_BINDIR)) go get ${go.fqp.misspell}

# Default rule for gotools uses the name->path map for a generic 'go get' style build
gotool.%:
	$(eval TOOL = ${subst gotool.,,${@}})
	@echo "Building ${go.fqp.${TOOL}} -> $(TOOL)"
	@GOPATH=$(abspath $(GOTOOLS_GOPATH)) GOBIN=$(abspath $(GOTOOLS_BINDIR)) go install ${go.fqp.${TOOL}}

$(GOTOOLS_BINDIR)/%:
	$(eval TOOL = ${subst $(GOTOOLS_BINDIR)/,,${@}})
	@$(MAKE) -f gotools.mk gotool.$(TOOL)
