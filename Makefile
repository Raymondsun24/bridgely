MCP_BIN := mcp/bridget

.PHONY: all extension mcp watch package clean

all: extension mcp

extension:
	tsc -p ./

mcp:
	go -C mcp build -o bridget .

watch:
	tsc -watch -p ./

package:
	npx @vscode/vsce package --no-dependencies

clean:
	rm -rf out $(MCP_BIN)
