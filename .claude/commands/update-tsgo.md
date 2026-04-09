# Update typescript-go submodule

Update the typescript-go submodule to the latest upstream commit on `origin/main`, verify compatibility, and land it.

## Steps

1. **Fetch latest upstream**: `cd typescript-go && git fetch origin main --quiet`

2. **Show changelog**: Print `git log --oneline HEAD..origin/main` to show what changed upstream. If there are no new commits, stop and tell the user the submodule is already up-to-date.

3. **Check shim removability**: For every method and field listed in `shim/checker/extra-shim.json` (and any other `shim/*/extra-shim.json` files), check whether they have been exported (capitalized) in the new upstream. Use `git show origin/main:<path>` to inspect the latest source. Report which shims (if any) can be removed.

4. **Update submodule pointer**: `cd typescript-go && git checkout origin/main --detach`. IMPORTANT: Do NOT apply patches here — the submodule pointer must reference a clean upstream commit. Patches are applied by `just init` in CI.

5. **Regenerate shims**: `go run tools/gen_shims/main.go`

6. **Copy collections** (excluding test files): `mkdir -p internal/collections && find ./typescript-go/internal/collections -type f ! -name '*_test.go' -exec cp {} internal/collections/ \;`

7. **Update Go modules**: Run `go mod tidy` for the root module and all `shim/*/` submodules.

8. **Build**: `go build ./cmd/tsgonest/`

9. **Run tests**: `just test` — if tests fail, diagnose and fix before proceeding.

10. **Close Dependabot submodule PRs**: Search for open PRs that update the typescript-go submodule (e.g., from Dependabot) using `gh pr list --search "typescript-go" --state open`. Close any found with `gh pr close <number> --comment "Superseded by manual submodule update"`.

11. **Commit**: Stage all changed files (`typescript-go`, `go.mod`, `go.sum`, `shim/**`, `internal/collections/`, any `shim/*/go.mod` or `go.sum`). Commit with message: `chore: update typescript-go submodule to <short-hash>` and a body listing the upstream commits and any shim changes.

12. **Create or update PR**: If on a branch with an existing PR, push and update the PR body. Otherwise, create a new branch `chore/update-tsgo-<short-hash>`, push, and open a PR.

13. **Watch CI**: `gh pr checks <number> --watch` — wait for all checks to pass. If CI fails, diagnose the issue, fix, and re-push.

14. **Report**: Summarize what changed, which shims were affected, and whether any can be removed.
