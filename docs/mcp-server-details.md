# BookStack MCP Server

This MCP server provides an interface for AI agents to interact with BookStack, enabling capabilities like searching documentation, reading content, and listing books, while respecting user permissions.

## Features

- **Protocol Support**: Implements Model Context Protocol (MCP) via Stdio (default).
- **Security**:
  - **Service Level**: Validates `MCP_SERVICE_TOKEN` (optional).
  - **User Level**: Supports `member_token` argument in tools to access private content based on user permissions.
  - **Data Safety**: Read-only access by default (depending on implemented tools).

## Architecture

The MCP server runs as a standalone Go binary (`mcp-server`) that:

1. Loads BookStack configuration (`conf/app.conf`).
2. Connects to the BookStack database (MySQL).
3. Reuses existing BookStack models (`models` package) for consistent logic.
4. Exposes tools and resources via MCP.

## Tools

### 1. `list_books`

List books available to the user.

- **Arguments**:
  - `category` (string, optional): Filter by category/name (partial match).
  - `include_private` (boolean, optional): Deprecated, use `member_token` instead.
  - `member_token` (string, optional): User's API Token. If provided, returns public books PLUS private books accessible to the user.

### 2. `get_doc`

Retrieve document content.

- **Arguments**:
  - `doc_id` (number, optional): The ID of the document.
  - `identify` (string, optional): Document slug (requires `book_id`).
  - `book_id` (number, optional): Book ID (required if using `identify`).
  - `format` (string, optional): `markdown` (default), `html`, or `text`.
  - `member_token` (string, optional): User's API Token for permission check.

### 3. `search_docs`

Search for documents across the library.

- **Arguments**:
  - `query` (string, required): Search keywords.
  - `book_id` (number, optional): Restrict search to a specific book.
  - `member_token` (string, optional): User's API Token. If provided, includes private documents accessible to the user.

## Configuration

The server reads `conf/app.conf` relative to the working directory. Ensure the database configuration matches your BookStack instance.

**Environment Variables**:

- `MCP_SERVICE_TOKEN`: (Optional) If set, the server requires this token in the initialization headers (transport dependent).

## Usage

### Docker (Recommended)

You can run the MCP server inside the BookStack container or as a sidecar.

**Run inside existing container**:

```bash
docker exec -i bookstack-app ./mcp-server
```

**Run locally**:
Ensure `conf/app.conf` points to the correct database host (e.g., `127.0.0.1` if using port mapping).

```bash
./mcp-server
```

### Integration with AI Clients

Configure your AI client (e.g., Claude Desktop) to run the `mcp-server` command.

```json
{
  "mcpServers": {
    "bookstack": {
      "command": "docker",
      "args": ["exec", "-i", "bookstack-app", "./mcp-server"]
    }
  }
}
```
