package mcp

import (
	"context"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain/ai"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/aiuc"
)

// aiTools exposes the governed-AI usecases (ADR-025). Every call goes through
// aiuc, so the surface gate, metering, ledger and audit still apply.
func aiTools(svc *aiuc.Service) []Tool {
	idemKey := func(c Caller, args Args) (string, error) {
		k, err := args.Str("idempotency_key")
		if err != nil || k != "" {
			return k, err
		}
		return c.IdempotencyKey, nil
	}
	return []Tool{
		{
			Name:        "ai.parse",
			Title:       "Parse text into task drafts",
			Description: "Turn a sentence or paragraph into structured task drafts (title, due date, assignee if inferable). Creates nothing.",
			InputSchema: objectSchema(map[string]any{"input": str("Free text describing the work.")}, "input"),
			Call: func(ctx context.Context, c Caller, args Args) (map[string]any, error) {
				input, err := args.RequiredStr("input")
				if err != nil {
					return nil, err
				}
				out, err := svc.Parse(ctx, c.OrgID, c.UserID, input)
				if err != nil {
					return nil, err
				}
				return map[string]any{"run_id": out.RunID, "model": out.Model, "items": out.Items}, nil
			},
		},
		{
			Name:        "ai.plan_draft",
			Title:       "Draft a plan",
			Description: "Draft a plan (tasks, subtasks, milestones, risks) from a brief. Creates nothing; apply it with ai.plan_apply.",
			InputSchema: objectSchema(map[string]any{
				"brief":           str("The brief, transcript or goal to plan."),
				"idempotency_key": str("Optional key; repeating it returns the same draft."),
			}, "brief"),
			Call: func(ctx context.Context, c Caller, args Args) (map[string]any, error) {
				brief, err := args.RequiredStr("brief")
				if err != nil {
					return nil, err
				}
				key, err := idemKey(c, args)
				if err != nil {
					return nil, err
				}
				out, err := svc.PlanDraft(ctx, c.OrgID, c.UserID, brief, key)
				if err != nil {
					return nil, err
				}
				return map[string]any{"run_id": out.RunID, "replayed": out.Replayed, "text": out.Text, "items": out.Items}, nil
			},
		},
		{
			Name:        "ai.plan_apply",
			Title:       "Create tasks from a plan",
			Description: "Create up to 50 tasks in one project column, all or none. Requires an idempotency key so a retry never duplicates tasks.",
			InputSchema: objectSchema(map[string]any{
				"project_id":      str("Project id."),
				"column_id":       str("Column id the tasks are created in."),
				"idempotency_key": str("Required unless the Idempotency-Key header is set."),
				"items": map[string]any{
					"type":     "array",
					"minItems": 1,
					"maxItems": 50,
					"items": objectSchema(map[string]any{
						"title":       str("1–200 characters."),
						"description": str("Optional."),
						"assignee_id": str("Optional user id."),
						"priority":    map[string]any{"type": "string", "enum": []string{"none", "low", "medium", "high", "urgent"}},
						"start_date":  map[string]any{"type": "string", "format": "date-time"},
						"due_date":    map[string]any{"type": "string", "format": "date-time"},
					}, "title"),
				},
			}, "project_id", "column_id", "items"),
			Write:      true,
			Idempotent: true,
			Call: func(ctx context.Context, c Caller, args Args) (map[string]any, error) {
				projectID, err := args.RequiredID("project_id")
				if err != nil {
					return nil, err
				}
				columnID, err := args.RequiredID("column_id")
				if err != nil {
					return nil, err
				}
				key, err := idemKey(c, args)
				if err != nil {
					return nil, err
				}
				items, err := applyItems(args)
				if err != nil {
					return nil, err
				}
				out, err := svc.ApplyPlan(ctx, c.OrgID, c.UserID, c.Role, aiuc.ApplyInput{
					ProjectID: projectID, ColumnID: columnID, Items: items, IdemKey: key,
				})
				if err != nil {
					return nil, err
				}
				return map[string]any{"run_id": out.RunID, "replayed": out.Replayed, "tasks": out.Tasks}, nil
			},
		},
		{
			Name:        "ai.digest",
			Title:       "Project status digest",
			Description: "A status report for one project computed from live data (numbers by the engine, prose by the model).",
			InputSchema: objectSchema(map[string]any{"project_id": str("Project id.")}, "project_id"),
			Call: func(ctx context.Context, c Caller, args Args) (map[string]any, error) {
				projectID, err := args.RequiredID("project_id")
				if err != nil {
					return nil, err
				}
				out, err := svc.Digest(ctx, c.OrgID, c.UserID, c.Role, projectID)
				if err != nil {
					return nil, err
				}
				return map[string]any{"run_id": out.RunID, "text": out.Text, "stats": out.Stats}, nil
			},
		},
		{
			Name:        "ai.chat",
			Title:       "Ask about the workspace",
			Description: "Ask a question about the organization's work; answers come from org data only.",
			InputSchema: objectSchema(map[string]any{
				"message": str("The question."),
				"history": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Earlier turns, oldest first."},
			}, "message"),
			Call: func(ctx context.Context, c Caller, args Args) (map[string]any, error) {
				msg, err := args.RequiredStr("message")
				if err != nil {
					return nil, err
				}
				history, err := args.Strs("history")
				if err != nil {
					return nil, err
				}
				out, err := svc.Chat(ctx, c.OrgID, c.UserID, msg, history)
				if err != nil {
					return nil, err
				}
				return map[string]any{"run_id": out.RunID, "text": out.Text}, nil
			},
		},
		{
			Name:        "ai.risks_scan",
			Title:       "Scan project risks",
			Description: "Recompute risk signals (overdue, overcommit, dependency churn) for a project and return the open ones.",
			InputSchema: objectSchema(map[string]any{"project_id": str("Project id.")}, "project_id"),
			Call: func(ctx context.Context, c Caller, args Args) (map[string]any, error) {
				projectID, err := args.RequiredID("project_id")
				if err != nil {
					return nil, err
				}
				risks, err := svc.ScanRisks(ctx, c.OrgID, c.UserID, c.Role, projectID)
				if err != nil {
					return nil, err
				}
				return map[string]any{"risks": riskViews(risks)}, nil
			},
		},
		{
			Name:        "ai.risks_list",
			Title:       "List open risks",
			Description: "List the open (not dismissed) risk signals of a project.",
			InputSchema: objectSchema(map[string]any{"project_id": str("Project id.")}, "project_id"),
			Call: func(ctx context.Context, c Caller, args Args) (map[string]any, error) {
				projectID, err := args.RequiredID("project_id")
				if err != nil {
					return nil, err
				}
				risks, err := svc.ListRisks(ctx, c.OrgID, c.UserID, c.Role, projectID)
				if err != nil {
					return nil, err
				}
				return map[string]any{"risks": riskViews(risks)}, nil
			},
		},
		{
			Name:        "ai.risks_dismiss",
			Title:       "Dismiss a risk",
			Description: "Dismiss the open risk signal on a task; it will not be raised again for the same cause.",
			InputSchema: objectSchema(map[string]any{"task_id": str("Task id the risk belongs to.")}, "task_id"),
			Write:       true,
			Idempotent:  true,
			Call: func(ctx context.Context, c Caller, args Args) (map[string]any, error) {
				taskID, err := args.RequiredID("task_id")
				if err != nil {
					return nil, err
				}
				if err := svc.DismissRisk(ctx, c.OrgID, c.UserID, c.Role, taskID); err != nil {
					return nil, err
				}
				return map[string]any{"dismissed": true}, nil
			},
		},
	}
}

// applyItems decodes ai.plan_apply items. Dates are RFC3339; a bad element is
// an argument error (never a silent default).
func applyItems(args Args) ([]aiuc.ApplyItem, error) {
	raw, ok := args["items"].([]any)
	if !ok || len(raw) == 0 {
		return nil, argErrorf("items must be a non-empty array")
	}
	out := make([]aiuc.ApplyItem, 0, len(raw))
	for i, v := range raw {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, argErrorf("items[%d] must be an object", i)
		}
		item := Args(m)
		title, err := item.Str("title")
		if err != nil {
			return nil, argErrorf("items[%d].%s", i, err.Error())
		}
		desc, _ := item.Str("description")
		priority, _ := item.Str("priority")
		var assignee *string
		s, err := item.ID("assignee_id")
		if err != nil {
			return nil, argErrorf("items[%d].%s", i, err.Error())
		}
		if s != "" {
			assignee = &s
		}
		date := func(key string) (*time.Time, error) {
			s, err := item.Str(key)
			if err != nil || s == "" {
				return nil, err
			}
			t, err := time.Parse(time.RFC3339, s)
			if err != nil {
				return nil, argErrorf("items[%d].%s must be an RFC 3339 date-time", i, key)
			}
			return &t, nil
		}
		start, err := date("start_date")
		if err != nil {
			return nil, err
		}
		due, err := date("due_date")
		if err != nil {
			return nil, err
		}
		out = append(out, aiuc.ApplyItem{
			Title: title, Description: desc, AssigneeID: assignee,
			Priority: priority, StartDate: start, DueDate: due,
		})
	}
	return out, nil
}

func riskViews(risks []ai.Risk) []map[string]any {
	out := make([]map[string]any, 0, len(risks))
	for i := range risks {
		r := &risks[i]
		signals := make([]map[string]any, 0, len(r.Signals))
		for _, s := range r.Signals {
			signals = append(signals, map[string]any{"code": s.Code, "detail": s.Detail})
		}
		out = append(out, map[string]any{
			"id": r.ID, "task_id": r.TaskID, "project_id": r.ProjectID, "score": r.Score,
			"signals": signals, "created_at": r.CreatedAt, "updated_at": r.UpdatedAt,
		})
	}
	return out
}
