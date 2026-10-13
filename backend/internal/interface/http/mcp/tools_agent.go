package mcp

import (
	"context"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/projectuc"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/taskuc"
)

// Agent write tools (ADR-033 decisions 5–6, FR-AGT-006..008). They author rows
// as the calling agent, so org keys never see them. Edits and moves need the
// agent's live claim and the version it read; work goes as far as review —
// only a person moves it to done.

const (
	maxSummaryLen  = 5000
	maxCommentLen  = 10000
	maxNewSubtasks = 50
)

var editableRoles = []string{string(project.ColumnBacklog), string(project.ColumnTodo), string(project.ColumnInProgress)}

// requireClaim loads a task and refuses it unless the caller holds a live
// claim on it.
func requireClaim(ctx context.Context, tasks *taskuc.Service, c Caller, taskID string) (*project.Task, error) {
	t, err := tasks.GetTask(ctx, c.OrgID, c.UserID, taskID, c.Role)
	if err != nil {
		return nil, err
	}
	if !t.ClaimActive(time.Now()) || *t.ClaimedBy != c.AgentID {
		return nil, project.ErrClaimLost
	}
	return t, nil
}

// editableRole parses an optional column role an agent may move to directly.
func editableRole(args Args, key string) (project.ColumnRole, error) {
	s, err := args.Str(key)
	if err != nil || s == "" {
		return "", err
	}
	switch project.ColumnRole(s) {
	case project.ColumnBacklog, project.ColumnTodo, project.ColumnInProgress:
		return project.ColumnRole(s), nil
	case project.ColumnReview:
		return "", argErrorf("%s: use tasks.submit_for_review to send work to review", key)
	case project.ColumnDone:
		return "", project.ErrHumanApprovalRequired
	}
	return "", argErrorf("%s must be one of backlog, todo, in_progress", key)
}

// optDate parses an optional date argument: absent ⇒ keep (ok=false), "" ⇒
// clear, else RFC 3339 or YYYY-MM-DD.
func optDate(args Args, key string) (val *time.Time, ok bool, err error) {
	if _, present := args[key]; !present {
		return nil, false, nil
	}
	s, err := args.Str(key)
	if err != nil {
		return nil, false, err
	}
	if s == "" {
		return nil, true, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, perr := time.Parse(layout, s); perr == nil {
			t = t.UTC()
			return &t, true, nil
		}
	}
	return nil, false, argErrorf("%s must be a date (YYYY-MM-DD) or RFC 3339 time", key)
}

func priorityArg(args Args, key string) (project.Priority, bool, error) {
	s, err := args.Str(key)
	if err != nil || s == "" {
		return "", false, err
	}
	p := project.Priority(s)
	if !p.Valid() {
		return "", false, argErrorf("%s must be one of urgent, high, medium, low, none", key)
	}
	return p, true, nil
}

func priorityEnum(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc, "enum": []string{"urgent", "high", "medium", "low", "none"}}
}

func strArray(desc string, maxItems int) map[string]any {
	return map[string]any{"type": "array", "description": desc, "items": map[string]any{"type": "string"}, "maxItems": maxItems}
}

func agentTools(projects *projectuc.Service, tasks *taskuc.Service) []Tool {
	return []Tool{
		{
			Name:  "tasks.next",
			Title: "Take the next task",
			Description: "Find your next ready task in a project and claim it (pass claim:false to only look). " +
				"Work sent back to you comes first, then todo tasks that are unassigned or assigned to you; " +
				"ready means no other agent holds it and every blocker is done. Most urgent first. " +
				"Returns task:null when nothing is ready.",
			InputSchema: objectSchema(map[string]any{
				"project_id":    str("Project id."),
				"claim":         boolean("Claim the task (default true)."),
				"lease_minutes": integer("Lease length in minutes (default 30).", 1, 240),
			}, "project_id"),
			Write: true, AgentOnly: true,
			Call: func(ctx context.Context, c Caller, args Args) (map[string]any, error) {
				projectID, err := args.RequiredID("project_id")
				if err != nil {
					return nil, err
				}
				claim := true
				if _, set := args["claim"]; set {
					if claim, err = args.Bool("claim"); err != nil {
						return nil, err
					}
				}
				m, err := args.Int("lease_minutes", 0)
				if err != nil {
					return nil, err
				}
				t, err := tasks.NextTask(ctx, c.OrgID, c.AgentID, projectID, claim, time.Duration(m)*time.Minute, c.Role)
				if err != nil {
					return nil, err
				}
				if t == nil {
					return map[string]any{"task": nil, "message": "No ready task right now."}, nil
				}
				return map[string]any{"task": taskSummary(t)}, nil
			},
		},
		{
			Name:  "tasks.update",
			Title: "Edit a task you hold",
			Description: "Change a task you have claimed: any of title, description, priority, start_date, due_date, " +
				"and/or move it to another stage (column: backlog, todo, in_progress). Pass the version you " +
				"last read; if the task changed since, nothing is written and you get the new version. " +
				"To hand work over use tasks.submit_for_review; only a person moves a task to done.",
			InputSchema: objectSchema(map[string]any{
				"task_id":     str("Task id."),
				"version":     integer("The task version you last read (tasks.get / tasks.claim).", 1, 1<<30),
				"title":       str("New title (max 200 characters)."),
				"description": str("New description (Markdown); replaces the old one."),
				"priority":    priorityEnum("New priority."),
				"start_date":  str("YYYY-MM-DD, or empty to clear."),
				"due_date":    str("YYYY-MM-DD, or empty to clear."),
				"column":      map[string]any{"type": "string", "enum": editableRoles, "description": "Move to this stage."},
			}, "task_id", "version"),
			Write: true, AgentOnly: true,
			Call: func(ctx context.Context, c Caller, args Args) (map[string]any, error) {
				taskID, err := args.RequiredID("task_id")
				if err != nil {
					return nil, err
				}
				v, err := args.Int("version", 0)
				if err != nil {
					return nil, err
				}
				if v < 1 {
					return nil, argErrorf("version is required")
				}
				version := int64(v)
				role, err := editableRole(args, "column")
				if err != nil {
					return nil, err
				}
				t, err := requireClaim(ctx, tasks, c, taskID)
				if err != nil {
					return nil, err
				}
				if t.Version != version {
					return nil, &project.VersionConflict{Current: t}
				}
				in := taskuc.UpdateTaskInput{
					Title: t.Title, Description: t.Description, AssigneeID: t.AssigneeID,
					Priority: t.Priority, StartDate: t.StartDate, DueDate: t.DueDate, IfVersion: &version,
				}
				changed := false
				if s, err := args.Str("title"); err != nil {
					return nil, err
				} else if s != "" {
					in.Title, changed = s, true
				}
				if _, present := args["description"]; present {
					s, err := args.Str("description")
					if err != nil {
						return nil, err
					}
					in.Description, changed = s, true
				}
				if p, ok, err := priorityArg(args, "priority"); err != nil {
					return nil, err
				} else if ok {
					in.Priority, changed = p, true
				}
				if d, ok, err := optDate(args, "start_date"); err != nil {
					return nil, err
				} else if ok {
					in.StartDate, changed = d, true
				}
				if d, ok, err := optDate(args, "due_date"); err != nil {
					return nil, err
				} else if ok {
					in.DueDate, changed = d, true
				}
				if !changed && role == "" {
					return nil, argErrorf("nothing to change: pass at least one field or column")
				}
				if changed {
					if t, err = tasks.UpdateTask(ctx, c.OrgID, c.UserID, taskID, in, c.Role); err != nil {
						return nil, err
					}
					version = t.Version
				}
				if role != "" {
					if _, err := projects.MoveTaskToRole(ctx, c.OrgID, c.UserID, taskID, role, &version, c.Role); err != nil {
						return nil, err
					}
					if t, err = tasks.GetTask(ctx, c.OrgID, c.UserID, taskID, c.Role); err != nil {
						return nil, err
					}
				}
				return map[string]any{"task": taskSummary(t)}, nil
			},
		},
		{
			Name:        "tasks.comment",
			Title:       "Comment on a task",
			Description: "Add a comment (Markdown) to a task: progress notes, questions, findings. @mentions notify people.",
			InputSchema: objectSchema(map[string]any{
				"task_id": str("Task id."),
				"body":    str("Comment text (max 10000 characters)."),
			}, "task_id", "body"),
			Write: true, AgentOnly: true,
			Call: func(ctx context.Context, c Caller, args Args) (map[string]any, error) {
				taskID, err := args.RequiredID("task_id")
				if err != nil {
					return nil, err
				}
				body, err := args.RequiredStr("body")
				if err != nil {
					return nil, err
				}
				if len([]rune(body)) > maxCommentLen {
					return nil, argErrorf("body is longer than %d characters", maxCommentLen)
				}
				cm, err := tasks.AddComment(ctx, c.OrgID, c.UserID, taskID, body, c.Role)
				if err != nil {
					return nil, err
				}
				return map[string]any{"comment_id": cm.ID, "task_id": taskID, "created_at": cm.CreatedAt}, nil
			},
		},
		{
			Name:  "tasks.create",
			Title: "Create a task",
			Description: "Create a task in a project, optionally with checklist subtasks and blockers. It lands in " +
				"the todo stage unless column says backlog or in_progress. It is not claimed: claim it to work on it.",
			InputSchema: objectSchema(map[string]any{
				"project_id":  str("Project id."),
				"title":       str("Title (max 200 characters)."),
				"description": str("Description (Markdown)."),
				"priority":    priorityEnum("Priority (default none)."),
				"due_date":    str("YYYY-MM-DD."),
				"column":      map[string]any{"type": "string", "enum": editableRoles, "description": "Stage (default todo)."},
				"subtasks":    strArray("Checklist items, in order.", maxNewSubtasks),
				"blocked_by":  strArray("Ids of tasks that must be done first.", 20),
			}, "project_id", "title"),
			Write: true, AgentOnly: true,
			Call: func(ctx context.Context, c Caller, args Args) (map[string]any, error) {
				projectID, err := args.RequiredID("project_id")
				if err != nil {
					return nil, err
				}
				title, err := args.RequiredStr("title")
				if err != nil {
					return nil, err
				}
				desc, err := args.Str("description")
				if err != nil {
					return nil, err
				}
				prio, _, err := priorityArg(args, "priority")
				if err != nil {
					return nil, err
				}
				due, _, err := optDate(args, "due_date")
				if err != nil {
					return nil, err
				}
				role, err := editableRole(args, "column")
				if err != nil {
					return nil, err
				}
				if role == "" {
					role = project.ColumnTodo
				}
				subs, err := args.Strs("subtasks")
				if err != nil {
					return nil, err
				}
				if len(subs) > maxNewSubtasks {
					return nil, argErrorf("at most %d subtasks", maxNewSubtasks)
				}
				blockers, err := args.Strs("blocked_by")
				if err != nil {
					return nil, err
				}
				for _, b := range blockers {
					if _, err := (Args{"id": b}).RequiredID("id"); err != nil {
						return nil, argErrorf("blocked_by must be task ids")
					}
				}
				col, err := projects.ColumnForRole(ctx, c.OrgID, c.UserID, projectID, role, c.Role)
				if err != nil {
					return nil, err
				}
				t, err := tasks.CreateTask(ctx, c.OrgID, c.UserID, taskuc.CreateTaskInput{
					ProjectID: projectID, ColumnID: col.ID, Title: title, Description: desc,
					Priority: prio, DueDate: due,
				}, c.Role)
				if err != nil {
					return nil, err
				}
				for _, st := range subs {
					if _, err := tasks.AddSubtask(ctx, c.OrgID, c.UserID, t.ID, st, c.Role); err != nil {
						return nil, err
					}
				}
				for _, b := range blockers {
					if err := tasks.AddLink(ctx, c.OrgID, c.UserID, t.ID, b, c.Role); err != nil {
						return nil, err
					}
				}
				return map[string]any{"task": taskSummary(t), "subtasks": len(subs), "blocked_by": blockers}, nil
			},
		},
		{
			Name:        "tasks.link",
			Title:       "Add a blocker",
			Description: "Record that task_id cannot be done before blocked_by_task_id is done.",
			InputSchema: objectSchema(map[string]any{
				"task_id":            str("The task that waits."),
				"blocked_by_task_id": str("The task it waits for."),
			}, "task_id", "blocked_by_task_id"),
			Write: true, Idempotent: true,
			Call: func(ctx context.Context, c Caller, args Args) (map[string]any, error) {
				taskID, err := args.RequiredID("task_id")
				if err != nil {
					return nil, err
				}
				blocker, err := args.RequiredID("blocked_by_task_id")
				if err != nil {
					return nil, err
				}
				if err := tasks.AddLink(ctx, c.OrgID, c.UserID, taskID, blocker, c.Role); err != nil {
					return nil, err
				}
				return map[string]any{"task_id": taskID, "blocked_by": blocker}, nil
			},
		},
		{
			Name:  "tasks.submit_for_review",
			Title: "Submit work for review",
			Description: "Hand a task you hold over to a person: it moves to the review stage, your summary is " +
				"posted as a comment, and your claim ends (you stay the assignee). A person then approves it " +
				"(done) or sends it back to you with a comment; tasks.next returns sent-back work first.",
			InputSchema: objectSchema(map[string]any{
				"task_id": str("Task id."),
				"version": integer("The task version you last read.", 1, 1<<30),
				"summary": str("What you did and how to check it (Markdown, max 5000 characters)."),
			}, "task_id", "version", "summary"),
			Write: true, AgentOnly: true,
			Call: func(ctx context.Context, c Caller, args Args) (map[string]any, error) {
				taskID, err := args.RequiredID("task_id")
				if err != nil {
					return nil, err
				}
				v, err := args.Int("version", 0)
				if err != nil {
					return nil, err
				}
				if v < 1 {
					return nil, argErrorf("version is required")
				}
				summary, err := args.RequiredStr("summary")
				if err != nil {
					return nil, err
				}
				if len([]rune(summary)) > maxSummaryLen {
					return nil, argErrorf("summary is longer than %d characters", maxSummaryLen)
				}
				version := int64(v)
				t, err := requireClaim(ctx, tasks, c, taskID)
				if err != nil {
					return nil, err
				}
				if t.Version != version {
					return nil, &project.VersionConflict{Current: t}
				}
				col, err := projects.MoveTaskToRole(ctx, c.OrgID, c.UserID, taskID, project.ColumnReview, &version, c.Role)
				if err != nil {
					return nil, err
				}
				if _, err := tasks.AddComment(ctx, c.OrgID, c.UserID, taskID, "Submitted for review:\n\n"+summary, c.Role); err != nil {
					return nil, err
				}
				t, err = tasks.EndClaim(ctx, c.OrgID, c.AgentID, taskID, c.Role)
				if err != nil {
					return nil, err
				}
				return map[string]any{"task": taskSummary(t), "column": col.Name}, nil
			},
		},
	}
}
