# Contributing to tsgonest

Thank you for your interest in contributing. This document covers how to set up
the project locally, run tests, and submit changes.

## Table of contents

- [Prerequisites](#prerequisites)
- [Setting up the project](#setting-up-the-project)
- [Repository layout](#repository-layout)
- [Building and testing](#building-and-testing)
- [Commit conventions](#commit-conventions)
- [Submitting a pull request](#submitting-a-pull-request)
- [Working with the typescript-go submodule](#working-with-the-typescript-go-submodule)
- [Adding shim methods](#adding-shim-methods)

---

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.24+ | Build the CLI binary |
| Node.js | 22+ | Run e2e tests and docs |
| pnpm | 10+ | Manage the npm workspace |
| Git | any | Version control |

## Setting up the project

```bash
# Clone with submodules (typescript-go is required)
git clone --recurse-submodules https://github.com/tsgonest/tsgonest.git
cd tsgonest

# Install npm workspace dependencies
pnpm install

# Build the Go binary
go build -o tsgonest ./cmd/tsgonest
```

If you cloned without `--recurse-submodules`:

```bash
git submodule update --init --recursive
```

## Repository layout

```
cmd/tsgonest/       CLI entry point
internal/           Core Go packages (analyzer, codegen, openapi, …)
packages/           npm packages (core, runtime, types, cli-*)
apps/docs/          Documentation site (Next.js + Fumadocs)
e2e/                End-to-end tests (Vitest)
testdata/           TypeScript fixtures used by Go unit tests
patches/            Git patches applied to typescript-go during init
shim/               Generated Go ↔ tsgo bindings — do not edit directly
typescript-go/      Microsoft's TypeScript Go port (Git submodule)
```

## Building and testing

```bash
# Go unit tests (fast, no binary required)
go test ./internal/... -count=1

# Build the binary (required before e2e)
go build -o tsgonest ./cmd/tsgonest

# End-to-end tests
pnpm test:e2e

# Build TypeScript packages
pnpm build:packages

# Docs dev server
pnpm dev:docs
```

Running just a single Go test package:

```bash
go test ./internal/analyzer/... -run TestTypeWalker -v
```

## Commit conventions

This project uses [Conventional Commits](https://www.conventionalcommits.org/).
The format is:

```
<type>(<optional scope>): <short description>

<optional body>

<optional footer>
```

| Type | When to use |
|------|-------------|
| `feat` | New user-facing feature |
| `fix` | Bug fix |
| `perf` | Performance improvement |
| `refactor` | Code change that neither fixes a bug nor adds a feature |
| `docs` | Documentation only |
| `test` | Adding or updating tests |
| `chore` | Build system, CI, dependency updates |
| `ci` | Changes to CI/CD workflows |

Breaking changes must include `BREAKING CHANGE:` in the footer or a `!` after
the type: `feat!: rename config field`.

The changelog is auto-generated from these commit messages on every release.

## Submitting a pull request

1. Fork the repo and create a branch from `main`.
2. Make your changes. Add or update tests as appropriate.
3. Ensure all tests pass: `go test ./internal/... && pnpm test:e2e`.
4. Push your branch and open a PR against `main`.
5. A maintainer will review and merge it.

For large changes, consider opening an issue first to discuss the approach.

## Working with the typescript-go submodule

`typescript-go/` is a Git submodule pointing to Microsoft's TypeScript Go port.
**Do not commit changes to it directly** — they will be lost when the submodule
is updated.

Instead, write a patch file:

```bash
# 1. Make your changes inside typescript-go/
cd typescript-go
# … edit files …

# 2. Create a patch
git format-patch HEAD~1 -o ../patches/

# 3. Reset the submodule
git checkout .

# 4. Back in the root: test that the patch applies cleanly
cd ..
git -C typescript-go am ../patches/0001-your-change.patch
```

Patches in `patches/` are applied automatically during `git submodule update`.

## Adding shim methods

tsgonest accesses unexported tsgo functions via a shim layer. To expose a new
method:

1. Find the function in `typescript-go/` (e.g. `checker.getTypeOfExpression`).
2. Add it to the relevant `shim/*/extra-shim.json` under `ExtraMethods`.
3. Regenerate: `go run tools/gen_shims/main.go`.
4. Use it in Go as `checker.Checker_getTypeOfExpression(recv, args...)`.
5. Run `go mod tidy` if new imports were added.

For more detail see `AGENTS.md`.
