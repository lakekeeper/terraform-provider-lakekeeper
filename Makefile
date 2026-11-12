default: reviewable

reviewable: build fmt generate test ## Run before committing.

GOBIN = $(shell pwd)/bin
PROVIDER_SRC_DIR := ./internal/provider/...
CONTAINER_COMPOSE_ENGINE ?= $(shell docker compose version >/dev/null 2>&1 && echo 'docker compose' || echo 'docker-compose')

build: ## Build the provider binary.
	go mod tidy
	GOBIN=$(GOBIN) go install

generate: ## Generate documentation.
	PATH="$(GOBIN):$(PATH)" go generate --tags lint ./...

ifdef RUN
TESTARGS += -test.run $(RUN)
endif

test: ## Run unit tests.
	go test -cover $(TESTARGS) $(PROVIDER_SRC_DIR)

fmt: tool-terraform ## Format files and fix issues.
	gofmt -s -w -e .
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint run --build-tags acceptance --fix
	$(GOBIN)/terraform fmt -recursive -list ./examples ./playground
	go run mvdan.cc/sh/v3/cmd/shfmt -l -s -w ./examples

lint: lint-golangci lint-examples-tf lint-examples-sh lint-actions lint-links lint-generated ## Run every linter without modifying files (what CI checks).

lint-golangci: ## Run golangci-lint linter (same as fmt but without modifying files).
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint run --build-tags acceptance

lint-examples-tf: tool-terraform ## Run terraform linter on examples (same as fmt but without modifying files).
	PATH="$(GOBIN):$(PATH)" terraform fmt -recursive -check -diff ./examples ./playground

lint-examples-sh: ## Run shell linter on examples (same as fmt but without modifying files).
	go run mvdan.cc/sh/v3/cmd/shfmt -l -s -d ./examples

# actionlint is not a go.mod tool: its YAML dependency conflicts with golangci-lint's.
# renovate: datasource=go depName=github.com/rhysd/actionlint
ACTIONLINT_VERSION ?= v1.7.12
# renovate: datasource=pypi depName=zizmor
ZIZMOR_VERSION ?= 1.32.0

lint-actions: lint-actionlint lint-zizmor ## Lint GitHub Actions workflows with actionlint and zizmor.

lint-actionlint: ## Lint GitHub Actions workflows with actionlint.
	go run github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION)

lint-zizmor: ## Audit GitHub Actions workflows with zizmor (requires uv).
	uvx zizmor@$(ZIZMOR_VERSION) .

zizmor-version: ## Print the pinned zizmor version (used by CI).
	@echo $(ZIZMOR_VERSION)

# renovate: datasource=docker depName=lycheeverse/lychee
LYCHEE_VERSION ?= 0.24.2

# Tracked Markdown, minus the release-please changelog and the generated graph.
LINK_CHECK_FILES = $(shell git ls-files '*.md' ':!:CHANGELOG.md' ':!:graphify-out/')

lint-links: ## Check relative links and anchors in tracked Markdown, offline (requires Docker).
	docker run --rm -v "$(CURDIR)":/input:ro -w /input lycheeverse/lychee:$(LYCHEE_VERSION) --config lychee.toml $(LINK_CHECK_FILES)

lint-links-files: ## Print the Markdown files the link check covers (used by CI).
	@echo $(LINK_CHECK_FILES)

lychee-version: ## Print the pinned lychee version (used by CI).
	@echo v$(LYCHEE_VERSION)

lint-generated: generate ## Check that "make generate" was called. Note this only works if the git workspace is clean.
	@echo "Checking git status"
	@[ -z "$(shell git status --short)" ] || { \
		echo "Error: Files should have been generated:"; \
		git status --short; echo "Diff:"; \
		git --no-pager diff HEAD; \
		echo "Run \"make generate\" and try again"; \
		exit 1; \
	}
	@echo "validate documentation"
	PATH="$(GOBIN):$(PATH)" go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs validate -provider-name lakekeeper

diagrams: ## Copy the newest Archify render of each diagram from .archify/ into architecture/ (does not run Archify).
	@./scripts/copy-diagrams.sh $(DIAGRAMS)

diagrams-render: ## Re-render diagrams from architecture/<slug>.json (DIAGRAMS="<slug> ...", default all; needs Archify, Chrome, Node).
	@./scripts/render-diagrams.sh $(DIAGRAMS)

LAKEKEEPER_ENDPOINT ?= http://localhost:8181
LAKEKEEPER_AUTH_URL ?= http://localhost:30080/realms/iceberg/protocol/openid-connect/token
LAKEKEEPER_CLIENT_ID ?= lakekeeper-admin
LAKEKEEPER_CLIENT_SECRET ?= KNjaj1saNq5yRidVEMdf1vI09Hm0pQaL

testacc-up: ## Launch a Lakekeeper instance.
	cd run; $(CONTAINER_COMPOSE_ENGINE) up -d
	LAKEKEEPER_ENDPOINT=$(LAKEKEEPER_ENDPOINT) LAKEKEEPER_AUTH_URL=$(LAKEKEEPER_AUTH_URL) LAKEKEEPER_CLIENT_ID=$(LAKEKEEPER_CLIENT_ID) LAKEKEEPER_CLIENT_SECRET=$(LAKEKEEPER_CLIENT_SECRET) ./scripts/await-healthy.sh
	
testacc-down: ## Teardown a Lakekeeper instance.
	cd run; $(CONTAINER_COMPOSE_ENGINE) down --volumes

testacc: ## Run acceptance tests against a Lakekeeper instance.
	TF_ACC=1 LAKEKEEPER_ENDPOINT=$(LAKEKEEPER_ENDPOINT) LAKEKEEPER_AUTH_URL=$(LAKEKEEPER_AUTH_URL) LAKEKEEPER_CLIENT_ID=$(LAKEKEEPER_CLIENT_ID) LAKEKEEPER_CLIENT_SECRET=$(LAKEKEEPER_CLIENT_SECRET) go test -cover --tags acceptance -v $(PROVIDER_SRC_DIR) $(TESTARGS) -timeout 40m

testacc-flakey: ## Run flakey acceptance tests against a Lakekeeper instance.
	TF_ACC=1 LAKEKEEPER_ENDPOINT=$(LAKEKEEPER_ENDPOINT) LAKEKEEPER_AUTH_URL=$(LAKEKEEPER_AUTH_URL) LAKEKEEPER_CLIENT_ID=$(LAKEKEEPER_CLIENT_ID) LAKEKEEPER_CLIENT_SECRET=$(LAKEKEEPER_CLIENT_SECRET) go test -cover --tags flakey -v $(PROVIDER_SRC_DIR) $(TESTARGS) -timeout 40m

testacc-settings: ## Run application settings acceptance tests against a Lakekeeper instance.
	TF_ACC=1 LAKEKEEPER_ENDPOINT=$(LAKEKEEPER_ENDPOINT) LAKEKEEPER_AUTH_URL=$(LAKEKEEPER_AUTH_URL) LAKEKEEPER_CLIENT_ID=$(LAKEKEEPER_CLIENT_ID) LAKEKEEPER_CLIENT_SECRET=$(LAKEKEEPER_CLIENT_SECRET) go test -cover --tags settings -v $(PROVIDER_SRC_DIR) $(TESTARGS) -timeout 40m

playground: tool-terraform build
	@cd playground; \
		TF_CLI_CONFIG_FILE=./.terraformrc; \
		$(GOBIN)/terraform apply -auto-approve

playground-destroy:
	@cd playground; \
		$(GOBIN)/terraform destroy -auto-approve; \
		rm -rf ./terraform.tfstate ./terraform.tfstate.backup .terraform .terraform.lock.hcl

# renovate: datasource=github-releases depName=hashicorp/terraform
TERRAFORM_VERSION = v1.17.2
tool-terraform:
	@# See https://github.com/hashicorp/terraform/issues/30356
	@[ -f $(GOBIN)/terraform ] || { mkdir -p tmp; cd tmp; rm -rf terraform; git clone --branch $(TERRAFORM_VERSION) --depth 1 https://github.com/hashicorp/terraform.git; cd terraform; GOBIN=$(GOBIN) go install; cd ../..; rm -rf tmp; }

# renovate: datasource=pypi depName=graphifyy
GRAPHIFY_VERSION ?= 0.9.108

GRAPHIFY = uvx --from 'graphifyy[terraform]==$(GRAPHIFY_VERSION)' graphify

graph: ## Rebuild the graphify knowledge graph in graphify-out/ (optional, requires uv).
	PYTHONHASHSEED=0 $(GRAPHIFY) update .

graph-setup: ## Register the graphify merge driver for graphify-out/graph.json in this clone (requires uv).
	git config merge.graphify.name "graphify graph.json union merge"
	git config merge.graphify.driver "$(GRAPHIFY) merge-driver %O %A %B"

clean: playground-destroy testacc-down
	go clean -testcache
	rm -rf bin/
