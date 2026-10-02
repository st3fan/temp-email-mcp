// temp-email-mcp is an MCP server that exposes read-only IMAP access to
// configured email accounts, mirroring the checkemail tool.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
)

const serverVersion = "0.1.0"

func main() {
	transport := flag.String("transport", "stdio", "transport to serve on: stdio, sse or http")
	addr := flag.String("addr", "localhost:8080", "listen address for the sse and http transports")
	flag.Parse()

	s := server.NewMCPServer("temp-email-mcp", serverVersion,
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)
	registerTools(s)

	switch *transport {
	case "stdio":
		if err := server.ServeStdio(s); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
			os.Exit(1)
		}
	case "sse":
		sse := server.NewSSEServer(s)
		fmt.Fprintf(os.Stderr, "temp-email-mcp serving sse on %s\n", *addr)
		if err := sse.Start(*addr); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
			os.Exit(1)
		}
	case "http":
		httpServer := server.NewStreamableHTTPServer(s)
		fmt.Fprintf(os.Stderr, "temp-email-mcp serving http on %s at /mcp\n", *addr)
		if err := httpServer.Start(*addr); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "ERROR: unknown transport %q (expected stdio, sse or http)\n", *transport)
		os.Exit(1)
	}
}
