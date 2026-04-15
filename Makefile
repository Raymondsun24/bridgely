MCP_BIN := mcp/bridgely

.PHONY: all extension mcp watch package clean

all: extension mcp

extension:
	npx tsc -p ./

mcp:
	go -C mcp build -o bridgely .

watch:
	npx tsc -watch -p ./

package:
	npx @vscode/vsce package --no-dependencies

clean:
	rm -rf out $(MCP_BIN)
