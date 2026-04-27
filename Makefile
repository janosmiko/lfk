.PHONY: setup lint lint-fix test coverage build generate-themes sonar bump-version refresh-vendor-hash release

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

# Recomputes vendorHash by setting it to lib.fakeHash, running `nix build`, and
# parsing the "got: sha256-..." line from the resulting hash mismatch. Run this
# whenever go.mod/go.sum changes -- the hash is content-addressed by the vendored
# module set, so any dep bump invalidates it.
refresh-vendor-hash: ## Recompute vendorHash in flake.nix from current go.sum (requires nix)
	@command -v nix >/dev/null 2>&1 || { \
		echo "error: nix is required to refresh vendorHash (https://nixos.org/download)"; exit 1; \
	}
	@cp flake.nix flake.nix.orig
	@sed -i.bak -E 's|^(\s*vendorHash = )"[^"]*";|\1"sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";|' flake.nix && rm flake.nix.bak
	@echo "Computing vendorHash via nix build (this may take a minute)..."
	@NEW_HASH=$$(nix build .#default --no-link --extra-experimental-features 'nix-command flakes' 2>&1 \
		| awk '/^[[:space:]]*got:/ {print $$2; exit}'); \
	if [ -z "$$NEW_HASH" ]; then \
		mv flake.nix.orig flake.nix; \
		echo "error: could not extract vendorHash from nix build output."; \
		echo "       try running 'nix build .#default' manually to diagnose."; \
		exit 1; \
	fi; \
	sed -i.bak -E "s|^(\s*vendorHash = )\"[^\"]*\";|\1\"$$NEW_HASH\";|" flake.nix && rm flake.nix.bak; \
	rm flake.nix.orig; \
	echo "Updated vendorHash to $$NEW_HASH"
	@grep -n "vendorHash" flake.nix

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
