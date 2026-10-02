# Toolchains run in Docker by default; override NPM/TF to use native binaries
# (CI does: `make worker-test NPM=npm`, `make tf-check TF=terraform`).
NODE_IMAGE ?= node:24
TF_IMAGE   ?= hashicorp/terraform:1.16.2
TF_DIR     := infra/terraform/cloudflare

VITE_APP_VERSION ?= dev
DOCKER_NPM_WEB = docker run --rm -v "$(CURDIR)/web":/src -w /src \
	-e VITE_APP_VERSION=$(VITE_APP_VERSION) $(NODE_IMAGE) npm
DOCKER_NPM_WORKER = docker run --rm -v "$(CURDIR)/worker":/src -w /src $(NODE_IMAGE) npm
NPM        ?= $(DOCKER_NPM_WEB)
NPM_WORKER ?= $(DOCKER_NPM_WORKER)
# TF_DATA_DIR keeps tf-check off any .terraform left by a real backend init.
TF         ?= docker run --rm -e TF_DATA_DIR=/w/.terraform-check -v "$(CURDIR)/$(TF_DIR)":/w -w /w $(TF_IMAGE)

.PHONY: web-lint web-test web-build assets worker-lint worker-test worker-deploy dev tf-check check

web-lint:
	cd web && $(NPM) ci && $(NPM) run lint

web-test:
	cd web && $(NPM) ci && $(NPM) run test

web-build:
	cd web && $(NPM) ci && $(NPM) run build

# The SPA is built with base /app/, so its output lands under public/app and
# the root-level icons/manifest it references are copied beside it. site/ holds
# the root-level discovery files (robots.txt, sitemap.xml, llms.txt, openapi.json).
assets: web-build
	rm -rf worker/public && mkdir -p worker/public/app
	cp -R web/dist/. worker/public/app/
	cp web/dist/favicon.ico web/dist/favicon.svg web/dist/apple-touch-icon.png \
	   web/dist/icon-192.png web/dist/icon-512.png web/dist/site.webmanifest worker/public/
	cp worker/site/* worker/public/

worker-lint:
	cd worker && $(NPM_WORKER) ci && $(NPM_WORKER) run lint

worker-test: assets
	cd worker && $(NPM_WORKER) ci && $(NPM_WORKER) run test

# Requires CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID in the environment.
worker-deploy: assets
	cd worker && $(NPM_WORKER) ci && \
	  $(NPM_WORKER) exec -- wrangler d1 migrations apply molla-prod --env prod --remote && \
	  $(NPM_WORKER) run deploy

# Local API + redirect on :8787 with a local D1; see web/README.md for the UI.
dev: assets
	cd worker && $(NPM_WORKER) ci && \
	  $(NPM_WORKER) exec -- wrangler d1 migrations apply molla-dev --local && \
	  docker run --rm -it -p 8787:8787 -v "$(CURDIR)/worker":/src -w /src \
	    -e PERMUTATION_KEY=molla-slice-1-fixed-test-key $(NODE_IMAGE) npm run dev

# fmt, init without a backend, validate: touches no Cloudflare account.
tf-check:
	$(TF) fmt -check -recursive
	$(TF) init -backend=false -input=false >/dev/null && $(TF) validate

# Everything CI verifies, in one command.
check: worker-lint worker-test web-lint web-test tf-check
