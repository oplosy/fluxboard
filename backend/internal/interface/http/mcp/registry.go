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

// Tools assembles the server's tool set.
func Tools(d Deps) []Tool {
	tools := workTools(d.Projects, d.Tasks)
	tools = append(tools, claimTools(d.Tasks)...)
	tools = append(tools, agentTools(d.Projects, d.Tasks)...)
	if d.AI != nil {
		tools = append(tools, aiTools(d.AI)...)
	}
	return tools
}
