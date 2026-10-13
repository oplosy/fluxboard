package mcp

import (
	"context"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/projectuc"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/taskuc"
)

// workTools are the read tools an agent needs to find and understand work
// (FR-AGT-001). They call the same usecases as the REST API, so project
// visibility, roles and RLS apply unchanged. Write tools arrive with claim and
// version control (ADR-033).
func workTools(projects *projectuc.Service, tasks *taskuc.Service) []Tool {
	return []Tool{
		{
			Name:        "projects.list",
			Title:       "List projects",
			Description: "Projects visible to this key, with their key and name. Archived projects are left out unless include_archived is true.",
			InputSchema: objectSchema(map[string]any{
				"include_archived": boolean("Also list archived (read-only) projects."),
			}),
			Call: func(ctx context.Context, c Caller, args Args) (map[string]any, error) {
				archived, err := args.Bool("include_archived")
				if err != nil {
					return nil, err
				}
				ps, err := projects.ListProjects(ctx, c.OrgID, c.UserID, c.Role, archived)
				if err != nil {
					return nil, err
				}
				out := make([]map[string]any, 0, len(ps))
				for i := range ps {
					p := &ps[i]
					out = append(out, map[string]any{
						"id": p.ID, "key": p.Key, "name": p.Name, "description": p.Description,
						"archived": p.Archived(), "updated_at": p.UpdatedAt,
					})
				}
				return map[string]any{"projects": out}, nil
			},
		},
		{
			Name:        "projects.board",
			Title:       "Read a project board",
			Description: "The project's columns in order, each with its tasks in board order.",
			InputSchema: objectSchema(map[string]any{"project_id": str("Project id (from projects.list).")}, "project_id"),
			Call: func(ctx context.Context, c Caller, args Args) (map[string]any, error) {
				projectID, err := args.RequiredID("project_id")
				if err != nil {
					return nil, err
				}
				b, err := projects.GetBoard(ctx, c.OrgID, c.UserID, projectID, c.Role)
				if err != nil {
					return nil, err
				}
				cols := make([]map[string]any, 0, len(b.Columns))
				for i := range b.Columns {
					cv := &b.Columns[i]
					ts := make([]map[string]any, 0, len(cv.Tasks))
					for j := range cv.Tasks {
						ts = append(ts, taskSummary(&cv.Tasks[j]))
					}
					col := map[string]any{"id": cv.Column.ID, "name": cv.Column.Name, "tasks": ts}
					if cv.Column.WIPLimit != nil {
						col["wip_limit"] = *cv.Column.WIPLimit
					}
					cols = append(cols, col)
				}
				return map[string]any{"project_id": projectID, "columns": cols}, nil
			},
		},
		{
			Name:  "tasks.search",
			Title: "Search tasks",
			Description: "Find tasks by text and/or filters (at least one is required). Results are paged; " +
				"total is the full match count.",
			InputSchema: objectSchema(map[string]any{
				"query":       str("Text matched against title and description."),
				"project_id":  str("Only this project."),
				"assignee_id": str("Only tasks assigned to this user id."),
				"column_id":   str("Only tasks in this column."),
				"priority":    map[string]any{"type": "string", "enum": []string{"none", "low", "medium", "high", "urgent"}},
				"label_id":    str("Only tasks with this label."),
				"limit":       integer("Page size (default 25).", 1, project.MaxSearchLimit),
				"offset":      integer("Items to skip.", 0, 100000),
			}),
			Call: func(ctx context.Context, c Caller, args Args) (map[string]any, error) {
				var f project.SearchFilter
				var err error
				if f.Query, err = args.Str("query"); err != nil {
					return nil, err
				}
				if f.Priority, err = args.Str("priority"); err != nil {
					return nil, err
				}
				for key, dst := range map[string]*string{
					"project_id": &f.ProjectID, "assignee_id": &f.AssigneeID,
					"column_id": &f.ColumnID, "label_id": &f.LabelID,
				} {
					if *dst, err = args.ID(key); err != nil {
						return nil, err
					}
				}
				if f.Query == "" && f.ProjectID == "" && f.AssigneeID == "" && f.ColumnID == "" && f.Priority == "" && f.LabelID == "" {
					return nil, argErrorf("give a query or at least one filter (project_id, assignee_id, column_id, priority, label_id)")
				}
				if f.Limit, err = args.Int("limit", 0); err != nil {
					return nil, err
				}
				if f.Offset, err = args.Int("offset", 0); err != nil {
					return nil, err
				}
				res, err := tasks.Search(ctx, c.OrgID, c.UserID, f, c.Role)
				if err != nil {
					return nil, err
				}
				out := make([]map[string]any, 0, len(res.Tasks))
				for i := range res.Tasks {
					out = append(out, taskSummary(&res.Tasks[i]))
				}
				return map[string]any{"total": res.Total, "tasks": out}, nil
			},
		},
		{
			Name:  "tasks.get",
			Title: "Read a task",
			Description: "One task with its description, subtasks, comments, and dependency links " +
				"(blocked_by must be done before this task; blocks waits on it).",
			InputSchema: objectSchema(map[string]any{"task_id": str("Task id.")}, "task_id"),
			Call: func(ctx context.Context, c Caller, args Args) (map[string]any, error) {
				taskID, err := args.RequiredID("task_id")
				if err != nil {
					return nil, err
				}
				t, err := tasks.GetTask(ctx, c.OrgID, c.UserID, taskID, c.Role)
				if err != nil {
					return nil, err
				}
				subs, err := tasks.ListSubtasks(ctx, c.OrgID, c.UserID, taskID, c.Role)
				if err != nil {
					return nil, err
				}
				comments, err := tasks.ListComments(ctx, c.OrgID, c.UserID, taskID, c.Role)
				if err != nil {
					return nil, err
				}
				links, err := tasks.ListLinks(ctx, c.OrgID, c.UserID, taskID, c.Role)
				if err != nil {
					return nil, err
				}
				detail := taskSummary(t)
				detail["description"] = t.Description
				detail["start_date"] = t.StartDate
				detail["sprint_id"] = t.SprintID
				detail["created_by"] = t.CreatedBy
				detail["created_at"] = t.CreatedAt

				subOut := make([]map[string]any, 0, len(subs))
				for i := range subs {
					subOut = append(subOut, map[string]any{"id": subs[i].ID, "title": subs[i].Title, "done": subs[i].Done})
				}
				comOut := make([]map[string]any, 0, len(comments))
				for i := range comments {
					cm := &comments[i]
					if cm.DeletedAt != nil {
						continue
					}
					comOut = append(comOut, map[string]any{
						"id": cm.ID, "author_id": cm.AuthorID, "body": cm.Body, "created_at": cm.CreatedAt,
					})
				}
				return map[string]any{
					"task":       detail,
					"subtasks":   subOut,
					"comments":   comOut,
					"blocked_by": linkViews(links.Blockers, false),
					"blocks":     linkViews(links.Blocked, true),
				}, nil
			},
		},
	}
}

func taskSummary(t *project.Task) map[string]any {
	return map[string]any{
		"id": t.ID, "project_id": t.ProjectID, "number": t.Number, "title": t.Title,
		"column_id": t.ColumnID, "assignee_id": t.AssigneeID, "priority": t.Priority,
		"due_date": dateOrNil(t.DueDate), "updated_at": t.UpdatedAt,
	}
}

// linkViews renders the other end of each edge. A blocker row names the other
// task in LinkedTaskID; a "blocks" row (this task is the blocker) in TaskID.
func linkViews(ls []project.TaskLink, otherIsTaskID bool) []map[string]any {
	out := make([]map[string]any, 0, len(ls))
	for i := range ls {
		l := &ls[i]
		other := l.LinkedTaskID
		if otherIsTaskID {
			other = l.TaskID
		}
		out = append(out, map[string]any{
			"task_id": other, "number": l.Number, "title": l.Title,
			"column_id": l.ColumnID, "project_id": l.ProjectID,
		})
	}
	return out
}

func dateOrNil(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC()
}
