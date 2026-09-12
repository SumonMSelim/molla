# Toolchains run in Docker by default; override GO/GOFMT/TF to use native
# binaries (CI does: `make test GO=go GOFMT=gofmt`, `make tf-check TF=terraform`).
GO_IMAGE ?= golang:1.27.1
TF_IMAGE ?= hashicorp/terraform:1.16.2
TF_DIR   := infra/terraform/aws

DOCKER_GO = docker run --rm -v "$(CURDIR)":/src -w /src \
	-v molla-mod:/go/pkg/mod -v molla-build:/root/.cache/go-build \
	-e GOFLAGS=-buildvcs=false $(GO_IMAGE)
GO    ?= $(DOCKER_GO) go
GOFMT ?= $(DOCKER_GO) gofmt
TF    ?= docker run --rm -v "$(CURDIR)/$(TF_DIR)":/w -w /w $(TF_IMAGE)

.PHONY: build test vet lint bench tf-check

build:
	$(GO) build ./...

test:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

lint:
	@files="$$(command find . -type f -name '*.go' -not -path './vendor/*')"; \
	out="$$($(GOFMT) -l $$files)"; \
	if [ -n "$$out" ]; then echo "$$out"; exit 1; fi

bench:
	$(GO) test ./... -run '^$$' -bench=. -benchmem

# fmt, init without a backend, validate: touches no cloud account.
tf-check:
	$(TF) fmt -check -recursive
	@cd $(TF_DIR) && for d in . modules/* envs/*; do \
	  [ -d "$$d" ] || continue; \
	  echo "== $(TF_DIR)/$$d"; \
	  $(TF) -chdir=$$d init -backend=false -input=false >/dev/null && $(TF) -chdir=$$d validate || exit 1; \
	done
