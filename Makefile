.PHONY: setup lint lint-fix test coverage build generate-themes sonar bump-version release

setup:
	git config core.hooksPath .githooks

bump-version: ## Bump flake.nix baseVersion to $(VERSION) (usage: make bump-version VERSION=0.9.23)
	@if [ -z "$(VERSION)" ]; then \
		echo "usage: make bump-version VERSION=X.Y.Z"; exit 1; \
	fi
	@if ! echo "$(VERSION)" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$$'; then \
		echo "error: VERSION must be X.Y.Z (no leading v, no suffix); got: $(VERSION)"; exit 1; \
	fi
	@sed -i.bak -E 's/^(\s*baseVersion = )"[0-9]+\.[0-9]+\.[0-9]+";/\1"$(VERSION)";/' flake.nix && rm flake.nix.bak
	@grep -n "baseVersion =" flake.nix

release: setup ## Bump version, commit, and create tag in one step (usage: make release VERSION=0.9.23)
	@if [ -z "$(VERSION)" ]; then \
		echo "usage: make release VERSION=X.Y.Z"; exit 1; \
	fi
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "error: working tree is not clean; commit or stash changes first"; \
		git status --short; \
		exit 1; \
	fi
	@if git rev-parse "v$(VERSION)" >/dev/null 2>&1; then \
		echo "error: tag v$(VERSION) already exists"; exit 1; \
	fi
	@$(MAKE) --no-print-directory bump-version VERSION=$(VERSION)
	@git add flake.nix
	@git commit -m "chore: bump version to v$(VERSION)"
	@git tag "v$(VERSION)"
	@echo ""
	@echo "Tag v$(VERSION) created. To publish:"
	@echo "  git push && git push --tags"

lint:
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...

test:
	go test ./...

coverage: ## Run tests with coverage report
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1
	go tool cover -html=coverage.out -o coverage.html
	@echo "Open coverage.html in your browser for details"

build: setup
	go build -o lfk .

GHOSTTY_THEMES_URL := https://deps.files.ghostty.org/ghostty-themes-release-20260216-151611-fc73ce3.tgz
GHOSTTY_THEMES_DIR := themes/ghostty

generate-themes: ## Download ghostty themes and regenerate colorschemes_gen.go
	@echo "Downloading ghostty themes..."
	@mkdir -p themes
	@curl -sL $(GHOSTTY_THEMES_URL) | tar xz -C themes/
	@echo "Generating colorschemes..."
	go run ./cmd/themegen --input-dir=$(GHOSTTY_THEMES_DIR) --output=internal/ui/colorschemes_gen.go
	@echo "Done. Run 'go test ./internal/ui/' to verify."
