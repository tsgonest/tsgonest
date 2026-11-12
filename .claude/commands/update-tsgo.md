# Update typescript submodule

Update the `typescript` submodule (microsoft/TypeScript, Go compiler in `typescript/tsc/`) to the latest upstream commit on `origin/main`, verify compatibility, and land it.

## Steps

1. **Fetch latest upstream**: `git -C typescript fetch --depth 1 origin main --quiet`. The repo is large; never fetch full history.

2. **Show changelog**: `gh api repos/microsoft/TypeScript/compare/<current-sha>...main --jq '.commits[] | .sha[0:9] + " " + (.commit.message | split("\n")[0])'`. Filter to commits touching `tsc/` if the list is long. If there are no new commits, stop and tell the user the submodule is already up-to-date.

3. **Check shim removability**: For every method and field listed in `shim/*/extra-shim.json`, check whether upstream has exported it (capitalized). Use `git -C typescript show origin/main:tsc/<path>`. Report which shims can be removed.

4. **Update submodule pointer**: `git -C typescript checkout --detach FETCH_HEAD`. Do NOT apply patches before staging: the pointer must reference a clean upstream commit. Patches are applied by `just init`.

5. **Re-apply patches for local work**: `cd typescript && git am --3way ../patches/*.patch`. If a patch fails, port it by hand, regenerate it with `git format-patch` into `patches/`, and reset.

6. **Regenerate shims**: `just shim`

7. **Update Go modules**: `go mod tidy` for the root module and every `shim/*/` module. Bump the `go` directive if `typescript/tsc/go.mod` requires a newer Go, and match `go-version` in `.github/workflows/*.yml`.

8. **Build**: `go build ./cmd/tsgonest/`

9. **Run tests**: `just test`. If tests fail, diagnose and fix before proceeding.

10. **Close Dependabot submodule PRs**: `gh pr list --search "typescript" --label submodule --state open`. Close any found with `gh pr close <number> --comment "Superseded by manual submodule update"`.

11. **Commit**: Check out the clean upstream commit in the submodule (`git -C typescript checkout --detach <upstream-sha>`), then stage `typescript`, `go.mod`, `go.sum`, `go.work.sum`, `shim/**`, `patches/` and any adapted Go code. Commit with message `chore: update typescript submodule to <short-hash>` and a body listing notable upstream commits and shim changes. Re-apply patches afterwards for local dev.

12. **Create or update PR**: If on a branch with an existing PR, push and update the PR body. Otherwise create branch `chore/update-tsgo-<short-hash>`, push, and open a PR.

13. **Watch CI**: `gh pr checks <number> --watch`. If CI fails, diagnose, fix, and re-push.

14. **Report**: Summarize what changed, which shims were affected, and whether any can be removed.
