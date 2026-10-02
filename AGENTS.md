# AGENTS.md

## Project

We are building an MCP server that can be used to create and use temporary
email accounts.

Long term it will run on a server that is also the mail server for a list of
domains managed by this tool. But the first iteration is simpler: expose the
functionality of the existing `../checkemail` tool as MCP tools. `checkemail`
is a read-only IMAP client (list, read, archive, folders, accounts) built for
AI agents, configured via `accounts.json`.

## Workflow

We work with issues and pull requests.

- When asked to build something, first write a GitHub issue describing it.
- The user iterates on the issue until satisfied.
- Only when the user says "go", implement it in a PR (or a stacked PR for larger work).

## Dependencies

This is a Go project.

- Prefer the Go standard library as much as possible.
- Do not reinvent things: pulling in well-supported third-party packages is fine when justified.
- The user is picky about dependencies and always makes the final decision on which specific packages to use. Propose packages, but do not add them without approval.

## Code style

- Do not write comments in code unless the code is genuinely complicated and needs an actual explanation.
- Do follow Go conventions: write a short doc comment above every function, in the style of the Go handbook (start with the function name, complete sentences).