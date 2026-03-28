// Package main provides the stc-mcp binary, an MCP server that exposes
// all STC toolchain operations as MCP tools over stdio transport.
package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "stc-mcp",
		Version: "1.0.0",
	}, nil)

	registerTools(server)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
