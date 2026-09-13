# Toolchains run in Docker by default; override GO/GOFMT/TF to use native
# binaries (CI does: `make test GO=go GOFMT=gofmt`, `make tf-check TF=terraform`).
GO_IMAGE   ?= golang:1.27.1
NODE_IMAGE ?= node:24-alpine
TF_IMAGE   ?= hashicorp/terraform:1.16.2
AWS_IMAGE  ?= amazon/aws-cli:2.32.9
TF_DIR     := infra/terraform/aws

DOCKER_GO = docker run --rm -v "$(CURDIR)":/src -w /src \
	-v molla-mod:/go/pkg/mod -v molla-build:/root/.cache/go-build \
	-e GOFLAGS=-buildvcs=false $(GO_IMAGE)
DOCKER_NPM = docker run --rm -v "$(CURDIR)/web":/src -w /src $(NODE_IMAGE) npm
GO    ?= $(DOCKER_GO) go
GOFMT ?= $(DOCKER_GO) gofmt
NPM   ?= $(DOCKER_NPM)
TF    ?= docker run --rm -v "$(CURDIR)/$(TF_DIR)":/w -w /w $(TF_IMAGE)
AWS   ?= docker run --rm -v "$(CURDIR)":/src -w /src \
	-v "$(HOME)/.aws":/root/.aws:ro -e AWS_PROFILE -e AWS_REGION $(AWS_IMAGE)

.PHONY: build test coverage vet lint bench tf-check build-lambda web-build web-lint web-test dev-api deploy-web

build:
	$(GO) build ./...

test:
	$(GO) test -race ./...

coverage:
	$(GO) test -race -covermode=atomic -coverprofile=coverage.out ./...

vet:
	$(GO) vet ./...

lint:
	@files="$$(command find . -type f -name '*.go' -not -path './vendor/*')"; \
	out="$$($(GOFMT) -l $$files)"; \
	if [ -n "$$out" ]; then echo "$$out"; exit 1; fi

bench:
	$(GO) test ./... -run '^$$' -bench=. -benchmem

build-lambda:
	@mkdir -p dist
	@for c in api redirect invalidate aggregate; do \
	  echo "lambda $$c"; \
	  docker run --rm -v "$(CURDIR)":/src -w /src \
	    -v molla-mod:/go/pkg/mod -v molla-build:/root/.cache/go-build \
	    -e GOFLAGS=-buildvcs=false -e GOOS=linux -e GOARCH=arm64 -e CGO_ENABLED=0 \
	    $(GO_IMAGE) go build -trimpath -ldflags='-s -w' -o dist/$$c/bootstrap ./cmd/aws/$$c; \
	  (cd dist/$$c && zip -FS -q ../$$c.zip bootstrap); \
	done

web-lint:
	cd web && $(NPM) ci && $(NPM) run lint

web-test:
	cd web && $(NPM) ci && $(NPM) run test

web-build:
	cd web && $(NPM) ci && $(NPM) run build

# Requires UI_BUCKET and DIST_ID; AWS credentials come from the host environment.
deploy-web:
	$(AWS) s3 sync web/dist "s3://$(UI_BUCKET)/app" --delete
	$(AWS) cloudfront create-invalidation --distribution-id "$(DIST_ID)" --paths '/app/*'

dev-api:
	docker run --rm -p 8080:8080 -v "$(CURDIR)":/src -w /src \
	  -v molla-mod:/go/pkg/mod -v molla-build:/root/.cache/go-build \
	  -e GOFLAGS=-buildvcs=false \
	  -e MOLLA_PERMUTATION_KEY=molla-slice-1-fixed-test-key \
	  -e MOLLA_PRIVACY_KEY=molla-local-privacy-key \
	  -e MOLLA_PUBLIC_BASE=http://127.0.0.1:8080 \
	  -e MOLLA_LISTEN=:8080 \
	  $(GO_IMAGE) go run ./cmd/server

# fmt, init without a backend, validate: touches no cloud account.
tf-check:
	$(TF) fmt -check -recursive
	@cd $(TF_DIR) && for d in . modules/* envs/*; do \
	  [ -d "$$d" ] || continue; \
	  echo "== $(TF_DIR)/$$d"; \
	  $(TF) -chdir=$$d init -backend=false -input=false >/dev/null && $(TF) -chdir=$$d validate || exit 1; \
	done
