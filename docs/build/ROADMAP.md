# Feature Roadmap — post-v1 gap closing

The v1 build (phases 0–7, see [README.md](README.md)) is complete. This file
tracks the features still missing compared with Jira and Monday.com, in the
order we intend to build them. It is the single place a new session reads to
know **what to build next** and **what is already in flight**.

---

## How to use this file (every session)

1. Find the first item whose status is `todo` in the lowest open wave. Items in
   a later wave may start early only when their **Depends on** items are `done`.
2. Before writing code, re-check the item against the codebase (it may already
   exist partly), then set its status to `in progress` and fill **Branch**.
3. Give it a requirement ID in [01-FUNCTIONAL-REQUIREMENTS.md](../01-FUNCTIONAL-REQUIREMENTS.md)
   (next free number in the matching section) and write that ID in **FR**.
   Architectural choices go to an ADR first (AGENTS.md §6).
4. When the PR merges, set the status to `done` and fill **PR**.
5. New ideas go to the **Unscheduled** list at the bottom, not into a wave,
   until someone decides where they fit.

Status values: `todo` · `in progress` · `done` · `dropped` (with a reason).

---

## Wave 1 — Planning foundations

Most later items build on these two.

| ID | Feature | Status | FR | Branch | PR |
|---|---|---|---|---|---|
| R-01 | Epics / task hierarchy | done | FR-PROJ-008 | `feat/epics` | oplosy/fluxboard#82 |
| R-02 | Story points / estimates | done | FR-TASK-015 | `feat/story-points` | oplosy/fluxboard#83 |

- **R-01 Epics / task hierarchy.** A task can have a parent (epic). Today
  subtasks are checklist items only, and there is no `parent_id` on tasks. Epic
  progress (done / total), filter and group the board by epic, show the epic on
  cards. *Inspired by:* Jira epics.
- **R-02 Story points / estimates.** An optional estimate on a task (points),
  summed per column, sprint and epic. *Inspired by:* Jira story points.

## Wave 2 — Planning on top of the foundations

| ID | Feature | Status | FR | Branch | PR | Depends on |
|---|---|---|---|---|---|---|
| R-03 | Sprint burndown and velocity | done | FR-AN-003 | `feat/sprint-burndown` | oplosy/fluxboard#84 | R-02 |
| R-04 | Board swimlanes | done | FR-PROJ-009 | `feat/board-swimlanes` | oplosy/fluxboard#85 | R-01 |
| R-05 | Releases / versions | done | FR-PROJ-010 | `feat/releases` | oplosy/fluxboard#86 | |
| R-06 | Workload / capacity view | done | FR-AN-004 | `feat/workload` | oplosy/fluxboard#87 | R-02 |

- **R-03 Sprint burndown and velocity.** Burndown per sprint and velocity over
  past sprints. Check what `analytics` already shows before starting.
- **R-04 Board swimlanes.** Split the board into horizontal lanes by assignee,
  epic or priority.
- **R-05 Releases / versions.** Assign tasks to a release, track its progress,
  generate release notes from done tasks. *Inspired by:* Jira versions.
- **R-06 Workload / capacity view.** Load per person (estimates and logged time
  from time tracking) over a date range. *Inspired by:* Monday workload.

## Wave 3 — Views

| ID | Feature | Status | FR | Branch | PR | Depends on |
|---|---|---|---|---|---|---|
| R-07 | Saved views and filters | done | FR-PROJ-011 | `feat/saved-views` | oplosy/fluxboard#88 | |
| R-08 | Editable table view | done | FR-PROJ-012 | `feat/editable-table` | oplosy/fluxboard#89 | |
| R-09 | Cross-project dashboards | done | FR-AN-005 | `claude/cross-project-dashboards-d0618e` | oplosy/fluxboard#91 | |

- **R-07 Saved views and filters.** Save board / list filters as personal or
  project-shared views.
- **R-08 Editable table view.** Edit cells in place in the list view (status,
  assignee, priority, due date, custom fields). *Inspired by:* Monday's main
  table.
- **R-09 Cross-project dashboards.** Widget dashboards across projects;
  analytics today is per project only.

## Wave 4 — Integrations

| ID | Feature | Status | FR | Branch | PR | Depends on |
|---|---|---|---|---|---|---|
| R-10 | GitHub pull-request linking | done | FR-TASK-016 | `claude/github-pr-linking-8cdab0` | oplosy/fluxboard#92 | |
| R-11 | Slack / Teams notifications | todo | | | | |

- **R-10 GitHub pull-request linking.** A task key such as `WEB-12` in a PR
  title, branch or commit links the PR to the task; merging can move the task.
  Today GitHub is used for sign-in only.
- **R-11 Slack / Teams notifications.** Deliver notifications to a channel as
  well as in-app and email.

## Wave 5 — Beyond tasks

| ID | Feature | Status | FR | Branch | PR | Depends on |
|---|---|---|---|---|---|---|
| R-12 | Goals / OKRs | done | FR-PROJ-014 | `claude/goals-0ce4ed` | oplosy/fluxboard#93 | R-01 |
| R-13 | Docs / wiki pages | done | FR-PROJ-013 | `feat/project-wiki` | oplosy/fluxboard#90 | |

- **R-12 Goals / OKRs.** Goals with progress computed from linked tasks or
  epics.
- **R-13 Docs / wiki pages.** Project pages that link to tasks.

---

## Unscheduled

Ideas not yet placed in a wave.

- iCal feed of due dates and sprints (token-protected, read-only).
- Capacity targets per person (points or hours per week) shown against the R-06
  workload, so overload is visible.
- Intake SLA: response-time target on intake requests, with an automation
  trigger when it is breached.
- Upgrade Next.js: the dev overlay and `pnpm build` flag a known vulnerability
  in 16.4.0 (a dependency change, needs approval).

## Done since v1

| Feature | FR | PR |
|---|---|---|
| Task watchers | FR-TASK-014 | oplosy/fluxboard#78 |
| Canvas redesign (board, list, task detail, shell) | — | oplosy/fluxboard#79 |
| Epics (R-01) | FR-PROJ-008 | oplosy/fluxboard#82 |
| Story points (R-02) | FR-TASK-015 | oplosy/fluxboard#83 |
| Sprint burndown and velocity (R-03) | FR-AN-003 | oplosy/fluxboard#84 |
| Board swimlanes (R-04) | FR-PROJ-009 | oplosy/fluxboard#85 |
| Releases (R-05) | FR-PROJ-010 | oplosy/fluxboard#86 |
| Workload view (R-06; capacity targets not included) | FR-AN-004 | oplosy/fluxboard#87 |
| Saved board views (R-07) | FR-PROJ-011 | oplosy/fluxboard#88 |
| Editable table view (R-08) | FR-PROJ-012 | oplosy/fluxboard#89 |
| Docs / wiki pages (R-13; flat, no history) | FR-PROJ-013 | oplosy/fluxboard#90 |
| Cross-project dashboards (R-09) | FR-AN-005 | oplosy/fluxboard#91 |
| GitHub pull-request linking (R-10; commits not scanned) | FR-TASK-016 | oplosy/fluxboard#92 |
| Goals (R-12; flat goals, no key results) | FR-PROJ-014 | oplosy/fluxboard#93 |
