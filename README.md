# Toba TODO

**todo is an issue tracker for you, your team, and your coding agents.** Instead of tracking tasks in a separate application, todo stores them right alongside your code as plain Markdown files. You can use the `todo` CLI to interact with your tasks, but more importantly, so can your favorite coding agent.

This gives your robot friends a juicy upgrade: now they get a complete view of your project, make suggestions for what to work on next, track their progress, create bug issues for problems they find, and more.

> Based on [hmans/beans](https://github.com/hmans/beans).

## Features

- **Track tasks, bugs, features**, and more right alongside your code.
- **Plain old Markdown files** stored in a `.issues` directory in your project. Easy to version control, readable and editable by humans and machines alike.
- Use the `todo` CLI to create, list, view, update, and archive issues; or **let your coding agent do it for you**.
- **Built-in GraphQL query engine** lets your agent get exactly the information it needs, keeping token use to a minimum.
- **Project memory** — completed issues are archived and serve as project memory that your coding agent can query for context about past work.
- **Sync to external trackers** — two-way sync with **ClickUp** and **GitHub Issues** via `todo sync`.
- **Interactive TUI** for browsing and managing issues from the terminal, with substring filtering (`/`), deep search (`//`), and sort picker (`o`).
- **Configurable editor** — set `editor` in `.todo.yml` to use a custom editor (supports multi-word commands like `code --wait`).
- **Markdown roadmap** — generate a roadmap document from your milestones and epics.

## Installation

Either download todo from the [Releases section](https://github.com/toba/todo/releases), or install it via Homebrew:

```bash
brew install toba/todo/todo
```

Alternatively, install directly via Go:

```bash
go install github.com/toba/todo@latest
```

## Configure Your Project

Inside the root directory of your project, run:

```bash
todo init
```

This creates a `.issues/` directory and a `.todo.yml` configuration file at the project root. Both are meant to be tracked in version control.

### Sample Configuration

```yaml
# .todo.yml
issues:
  path: .issues
  editor: "code --wait"

sync:
  # ClickUp integration (requires CLICKUP_TOKEN env var)
  clickup:
    list_id: "123456789"
    status_mapping:
      draft: "backlog"
      ready: "to do"
      in-progress: "in progress"
      completed: "complete"
      scrapped: "closed"

  # GitHub Issues integration (requires GITHUB_TOKEN env var)
  github:
    repo: "owner/repo"
```

## Agent Configuration

The most basic way to teach your agent about todo is to add the following instruction to your `AGENTS.md`, `CLAUDE.md`, or equivalent file:

```
**IMPORTANT**: before you do anything else, run the `todo prime` command and heed its output.
```

### Claude Code

Add the following hooks to your project's `.claude/settings.json` file:

```json
{
  "hooks": {
    "SessionStart": [
      { "hooks": [{ "type": "command", "command": "todo prime" }] }
    ],
    "PreCompact": [
      { "hooks": [{ "type": "command", "command": "todo prime" }] }
    ]
  }
}
```

## Usage

```bash
todo help          # List all commands
todo tui           # Interactive terminal UI
todo list          # List all issues
todo create "Fix login bug" -t bug -s ready
todo show abc-def  # View an issue
todo sync          # Sync to ClickUp or GitHub Issues
```

### Agent Workflows

The real power of todo comes from letting your coding agent manage tasks. Assuming you have integrated todo into your agent, you can use natural language:

```
Are there any tasks we should be tracking for this project? If so, please create issues for them.
```

```
What should we work on next?
```

```
It's time to tackle abc-def.
```

```
Please inspect this project's issues and reorganize them into epics and milestones.
```

## Syncing with External Trackers

todo syncs issues bidirectionally with **ClickUp** and **GitHub Issues**. Configure the integration in `.todo.yml` under `sync`, then run:

```bash
todo sync                  # Sync all issues
todo sync abc-def xyz-123  # Sync specific issues
todo sync --dry-run        # Preview changes without applying
todo sync --force          # Force update even if unchanged
```

### ClickUp

Requires `CLICKUP_TOKEN` environment variable. Syncs statuses, priorities, types, and blocking relationships as ClickUp task dependencies.

```yaml
sync:
  clickup:
    list_id: "123456789"           # Required
    assignee: 42                    # Optional: default assignee
    status_mapping:
      draft: "backlog"
      ready: "to do"
      in-progress: "in progress"
      completed: "complete"
      scrapped: "closed"
    priority_mapping:               # ClickUp: 1=Urgent, 2=High, 3=Normal, 4=Low
      critical: 1
      high: 2
      normal: 3
      low: 4
    custom_fields:
      bean_id: "cf-field-uuid"
      created_at: "cf-field-uuid"
      updated_at: "cf-field-uuid"
    sync_filter:
      exclude_status:
        - completed
        - scrapped
```

### GitHub Issues

Requires `GITHUB_TOKEN` environment variable. Maps statuses, priorities, and types to GitHub labels (e.g., `status:in-progress`, `priority:high`, `type:bug`). Blocking relationships are rendered as text in the issue body.

```yaml
sync:
  github:
    repo: "owner/repo"   # Required
```

## Sync

todo supports sync integrations for syncing with external systems. Per-issue sync state is stored in frontmatter:

```yaml
---
title: Fix login bug
status: ready
sync:
  clickup:
    task_id: "868h4hd05"
    synced_at: "2026-01-18T00:07:02Z"
  github:
    issue_number: "42"
    synced_at: "2026-01-18T00:07:02Z"
---
```

Sync data is readable and writable via the GraphQL API:

```graphql
# Read sync data
{ bean(id: "abc-def") { sync { name data } } }

# Write sync data
mutation { setSyncData(id: "abc-def", name: "clickup", data: { task_id: "xyz" }) { id } }

# Filter by sync
{ beans(filter: { hasSync: "clickup" }) { id title } }
{ beans(filter: { syncStale: "clickup" }) { id title } }
```

### Building a Sync Integration

Sync integrations are standalone programs that shell out to the `todo` CLI or use the GraphQL API directly. A typical integration:

1. Reads its config from `.todo.yml` (under `sync.<name>`)
2. Queries issues via `todo query --json '{ beans(filter: { syncStale: "myext" }) { ... } }'`
3. Syncs data with the external system
4. Writes back via `todo query 'mutation { setSyncData(...) { id } }'`

## License

This project is licensed under the Apache-2.0 License. See the [LICENSE](LICENSE) file for details.

## Getting in Touch

If you have questions, suggestions, or feedback, please [open an issue](https://github.com/toba/todo/issues).
