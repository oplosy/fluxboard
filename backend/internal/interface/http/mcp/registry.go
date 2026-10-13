package mcp

import (
	"github.com/mesutokul/fluxboard/backend/internal/usecase/aiuc"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/projectuc"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/taskuc"
)

// Deps are the usecases the tools call.
type Deps struct {
	AI       *aiuc.Service // nil ⇒ the ai.* tools are left out
	Projects *projectuc.Service
	Tasks    *taskuc.Service
}

// needsAgentIdentity are tools that create rows authored by the caller. An API
// key is not a user, so they cannot run over MCP until agents become
// principals (ADR-033 step 2); listing them would only offer a broken tool.
var needsAgentIdentity = map[string]bool{"ai.plan_apply": true}

// Tools assembles the server's tool set.
func Tools(d Deps) []Tool {
	tools := workTools(d.Projects, d.Tasks)
	if d.AI != nil {
		for _, t := range aiTools(d.AI) {
			if !needsAgentIdentity[t.Name] {
				tools = append(tools, t)
			}
		}
	}
	return tools
}
