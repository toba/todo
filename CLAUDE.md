# What we're building

You already know what todo is. This is the todo repository.

# Commits

- Use conventional commit messages ("feat", "fix", "chore", etc.) when making commits.
- Include the relevant issue ID(s) in the commit message (please follow conventional commit conventions, e.g. `Refs: xxxx`).
- Mark commits as "breaking" using the `!` notation when applicable (e.g., `feat!: ...`).
- When making commits, provide a meaningful commit message. The description should be a concise bullet point list of changes made.

# Pull Requests

- When we're working in a PR branch, make separate commits, and update the PR description to reflect the changes made.
- Include the relevant issue ID(s) in the PR title (please follow conventional commit conventions, e.g. `Refs: xxxx`).

# Project Specific

- When making changes to the GraphQL schema, run `mise codegen` to regenerate the code.
- The `internal/graph/` package provides a GraphQL resolver that can be used to query and mutate issues.
- All CLI commands that interact with issues should internally use GraphQL queries/mutations.
- `mise build` to build a `./todo` executable

# Extra rules for our own issues

- Use the `idea` tag for ideas and proposals.

# Testing

## Unit Tests

- Always write or update tests for the changes you make.
- Run all tests: `mise test`
- Run specific package: `go test ./internal/issue/`
- Use table-driven tests following Go conventions

## Manual CLI Testing

- `mise todo` will compile and run the todo CLI. Use it instead of building and running `./todo` manually.
- When testing read-only functionality, feel free to use this project's own `.issues/` directory. But for anything that modifies data, create a separate test project directory. All commands support the `--data-path` flag to specify a custom path.

# currentDate
Today's date is 2026-02-14.
