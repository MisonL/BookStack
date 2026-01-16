package mcp

import (

	// 使用 mark3labs/mcp-go 作为 MCP 协议实现

	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ServeStdio starts the MCP server in Stdio mode
func ServeStdio() error {
	s := createServer()
	return server.ServeStdio(s)
}

// ServeSSE starts the MCP server in SSE mode
func ServeSSE(addr string) error {
	/*
	   s := createServer()
	   sseServer := server.NewSSEServer(s, "http://"+addr)
	   return sseServer.Start(addr)
	*/
	return fmt.Errorf("SSE not implemented yet")
}

func createServer() *server.MCPServer {
	s := server.NewMCPServer(
		"BookStack MCP",
		"1.0.0",
		server.WithResourceCapabilities(true, true), // List, Read
		server.WithToolCapabilities(true),
	)

	// Tool: list_books
	s.AddTool(mcp.NewTool("list_books",
		mcp.WithDescription("List books in BookStack"),
		mcp.WithString("member_token", mcp.Description("User token for permission (optional)")),
	), HandleListBooks)

	// Tool: get_doc
	s.AddTool(mcp.NewTool("get_doc",
		mcp.WithDescription("Get document content"),
		mcp.WithNumber("doc_id", mcp.Description("Document ID")),
		mcp.WithString("identify", mcp.Description("Document Identify (alternative to doc_id)")),
		mcp.WithNumber("book_id", mcp.Description("Book ID (required if using identify)")),
		mcp.WithString("format", mcp.Description("Output format: markdown (default), html, text")),
		mcp.WithString("member_token", mcp.Description("User token for permission")),
	), HandleGetDoc)

	// Tool: search_docs
	s.AddTool(mcp.NewTool("search_docs",
		mcp.WithDescription("Search documents"),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query")),
		mcp.WithNumber("book_id", mcp.Description("Restrict search to a specific book")),
		mcp.WithString("member_token", mcp.Description("User token for permission")),
	), HandleSearchDocs)

	// Resources
	// s.AddResource(mcp.NewResource("bookstack://books", "List Books", mcp.WithMIMEType("application/json")), HandleResourceBooks)

	return s
}

// Placeholder Handlers (Implemented in tools.go)
// func HandleListBooks(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
// 	return mcp.NewToolResultError("Not implemented"), nil
// }

// func HandleGetDoc(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
// 	return mcp.NewToolResultError("Not implemented"), nil
// }

// func HandleSearchDocs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
// 	return mcp.NewToolResultError("Not implemented"), nil
// }

/*
func HandleResourceBooks(ctx context.Context, request mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
    // ...
     return &mcp.ReadResourceResult{
        Contents: []mcp.ResourceContent{
            {
                URI: request.Params.URI,
                MIMEType: "text/markdown",
                Text: "sb.String()", // Placeholder for actual content
            },
        },
     }, nil
}
*/
