package mcp

import (
	"context"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/taskuc"
)

// claimTools are the agent claim lifecycle (ADR-033 decision 3, FR-AGT-004):
// take a task, keep it alive while working, give it back. They act as the
// calling agent, so org keys never see them.
func claimTools(tasks *taskuc.Service) []Tool {
	lease := func(args Args) (time.Duration, error) {
		m, err := args.Int("lease_minutes", 0)
		if err != nil {
			return 0, err
		}
		return time.Duration(m) * time.Minute, nil
	}
	leaseSchema := integer("Lease length in minutes (default 30).",
		int(project.MinLease/time.Minute), int(project.MaxLease/time.Minute))
	return []Tool{
		{
			Name:  "tasks.claim",
			Title: "Claim a task",
			Description: "Take a task to work on: it is assigned to you and nobody else can claim it while your " +
				"lease runs (default 30 minutes). Renew with tasks.heartbeat; give it back with tasks.release. " +
				"Calling it again on your own task renews it. Fails, naming the holder, if another agent holds " +
				"a live claim or a person is assigned.",
			InputSchema: objectSchema(map[string]any{
				"task_id":       str("Task id."),
				"lease_minutes": leaseSchema,
			}, "task_id"),
			Write: true, Idempotent: true, AgentOnly: true,
			Call: func(ctx context.Context, c Caller, args Args) (map[string]any, error) {
				taskID, err := args.RequiredID("task_id")
				if err != nil {
					return nil, err
				}
				d, err := lease(args)
				if err != nil {
					return nil, err
				}
				t, err := tasks.ClaimTask(ctx, c.OrgID, c.AgentID, taskID, d, c.Role)
				if err != nil {
					return nil, err
				}
				return map[string]any{"task": taskSummary(t)}, nil
			},
		},
		{
			Name:  "tasks.heartbeat",
			Title: "Keep a claim alive",
			Description: "Extend your claim on a task you are still working on. Call it well before " +
				"lease_expires_at. Fails if your claim lapsed or was released: claim the task again.",
			InputSchema: objectSchema(map[string]any{
				"task_id":       str("Task id."),
				"lease_minutes": leaseSchema,
			}, "task_id"),
			Write: true, Idempotent: true, AgentOnly: true,
			Call: func(ctx context.Context, c Caller, args Args) (map[string]any, error) {
				taskID, err := args.RequiredID("task_id")
				if err != nil {
					return nil, err
				}
				d, err := lease(args)
				if err != nil {
					return nil, err
				}
				t, err := tasks.HeartbeatTask(ctx, c.OrgID, c.AgentID, taskID, d, c.Role)
				if err != nil {
					return nil, err
				}
				return map[string]any{"task": taskSummary(t)}, nil
			},
		},
		{
			Name:  "tasks.release",
			Title: "Release a claim",
			Description: "Give a task back without finishing it: your claim ends and, if you are still its " +
				"assignee, it is unassigned so another agent can take it.",
			InputSchema: objectSchema(map[string]any{"task_id": str("Task id.")}, "task_id"),
			Write:       true, AgentOnly: true,
			Call: func(ctx context.Context, c Caller, args Args) (map[string]any, error) {
				taskID, err := args.RequiredID("task_id")
				if err != nil {
					return nil, err
				}
				t, err := tasks.ReleaseTask(ctx, c.OrgID, c.AgentID, taskID, c.Role)
				if err != nil {
					return nil, err
				}
				return map[string]any{"task": taskSummary(t)}, nil
			},
		},
	}
}
