// Package mcp serves the Model Context Protocol on POST /api/v1/mcp over the
// Streamable HTTP transport (ADR-034, FR-AI-007, FR-AGT-001). It is dual-era:
// modern clients (2026-07-28) send the protocol version on every request in
// params._meta and the MCP-Protocol-Version header, with no handshake; legacy
// clients (2025-11-25 and earlier) open with initialize. Both are served
// statelessly from the same endpoint: no sessions are minted.
//
// Authentication and tenancy are done by the HTTP middleware chain before this
// handler runs: the caller is an org-bound API key, and every tool executes
// through the same usecases (and so the same RBAC, RLS and metering) as the
// REST API.
package mcp

import "encoding/json"

// Protocol revisions this server speaks.
const (
	// VersionModern is the stateless, per-request-metadata revision.
	VersionModern = "2026-07-28"
	// versionNoHeader is what a legacy request without MCP-Protocol-Version is
	// treated as (the header did not exist before 2025-06-18).
	versionNoHeader = "2025-03-26"
)

// legacyVersions are the handshake-based revisions served, newest first.
var legacyVersions = []string{"2025-11-25", "2025-06-18", "2025-03-26"}

// supportedVersions lists every revision, newest first (UnsupportedProtocolVersion data).
func supportedVersions() []string {
	return append([]string{VersionModern}, legacyVersions...)
}

func isLegacyVersion(v string) bool {
	for _, l := range legacyVersions {
		if l == v {
			return true
		}
	}
	return false
}

// Reserved _meta keys (2026-07-28 basic §_meta).
const (
	metaProtocolVersion    = "io.modelcontextprotocol/protocolVersion"
	metaClientCapabilities = "io.modelcontextprotocol/clientCapabilities"
	metaServerInfo         = "io.modelcontextprotocol/serverInfo"
)

// HTTP request metadata headers (Streamable HTTP §Request Metadata).
const (
	headerProtocolVersion = "MCP-Protocol-Version"
	headerMethod          = "Mcp-Method"
	headerName            = "Mcp-Name"
)

// JSON-RPC 2.0 and MCP error codes.
const (
	codeParseError         = -32700
	codeInvalidRequest     = -32600
	codeMethodNotFound     = -32601
	codeInvalidParams      = -32602
	codeInternalError      = -32603
	codeHeaderMismatch     = -32020
	codeUnsupportedVersion = -32022
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// isNotification reports a message without an id (no response is sent).
func (r rpcRequest) isNotification() bool { return r.ID == nil }

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// requestParams is the subset of params the dispatcher reads; tool arguments
// stay raw until a tool decodes them.
type requestParams struct {
	Meta            map[string]json.RawMessage `json:"_meta"`
	Name            string                     `json:"name"`
	Arguments       json.RawMessage            `json:"arguments"`
	ProtocolVersion string                     `json:"protocolVersion"` // legacy initialize
}

type implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// toolDescriptor is a Tool as listed by tools/list.
type toolDescriptor struct {
	Name        string           `json:"name"`
	Title       string           `json:"title,omitempty"`
	Description string           `json:"description"`
	InputSchema map[string]any   `json:"inputSchema"`
	Annotations *toolAnnotations `json:"annotations,omitempty"`
}

type toolAnnotations struct {
	Title           string `json:"title,omitempty"`
	ReadOnlyHint    bool   `json:"readOnlyHint"`
	DestructiveHint bool   `json:"destructiveHint"`
	IdempotentHint  bool   `json:"idempotentHint"`
	OpenWorldHint   bool   `json:"openWorldHint"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
