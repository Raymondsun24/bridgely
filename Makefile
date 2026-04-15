MCP_BIN := mcp/bridgely

.PHONY: all extension mcp watch package clean test lint

all: extension mcp

extension:
	npx tsc -p ./

mcp:
	go -C mcp build -o bridgely .

watch:
	npx tsc -watch -p ./

package:
	npx @vscode/vsce package --no-dependencies

test:
	npm test
	go -C mcp test ./... -v

lint:
	npm run lint
	go -C mcp vet ./...
	cd mcp && golangci-lint run ./...

clean:
	rm -rf out $(MCP_BIN)
