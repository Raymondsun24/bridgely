MCP_BIN  := mcp/bridgely
VERSION  := $(shell node -p "require('./package.json').version")
VSIX     := bridgely-$(VERSION).vsix

.PHONY: all extension mcp watch package install-extension jetbrains clean test lint help

# ── Default ───────────────────────────────────────────────────────────────────
# End-users only need `make mcp` — the extension is installed from the marketplace.
# Developers: run `make help` to see all available targets.

all: extension mcp

# ── End-user targets ──────────────────────────────────────────────────────────

## mcp: Build the Go MCP server binary (required for all users)
mcp:
	go -C mcp build -o bridgely .

# ── Developer targets ─────────────────────────────────────────────────────────

## extension: Compile the VS Code extension TypeScript (requires npm install first)
extension:
	npx tsc -p ./

## watch: Watch mode — recompile extension TypeScript on save
watch:
	npx tsc -watch -p ./

## package: Bundle the extension into a .vsix file for local install or publishing
package:
	npx @vscode/vsce package --no-dependencies

## install-extension: Package and install the extension directly into VS Code/Cursor
install-extension: package
	code --install-extension $(VSIX) || cursor --install-extension $(VSIX)

## jetbrains: Build the JetBrains plugin (requires Gradle wrapper bootstrapped first)
jetbrains:
	cd jetbrains-plugin && ./gradlew buildPlugin

## test: Run all tests (extension + MCP server)
test:
	npm test
	go -C mcp test ./... -v

## lint: Lint all code (extension + MCP server)
lint:
	npm run lint
	go -C mcp vet ./...
	cd mcp && golangci-lint run ./...

## clean: Remove build artifacts
clean:
	rm -rf out $(MCP_BIN) *.vsix

# ── Help ──────────────────────────────────────────────────────────────────────

## help: Show available targets
help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "End-user:"
	@echo "  mcp                Build the Go MCP server binary"
	@echo ""
	@echo "Developer:"
	@echo "  extension          Compile the VS Code extension (requires npm install)"
	@echo "  watch              Watch mode for extension TypeScript"
	@echo "  package            Bundle extension into a .vsix file"
	@echo "  install-extension  Package and install .vsix into VS Code/Cursor"
	@echo "  jetbrains          Build the JetBrains plugin"
	@echo "  test               Run all tests"
	@echo "  lint               Lint all code"
	@echo "  clean              Remove build artifacts"
