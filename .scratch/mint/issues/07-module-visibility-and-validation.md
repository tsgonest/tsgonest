# 07: Module visibility and boot validation

Status: ready-for-agent

## Parent

[../PRD.md](../PRD.md)

## What to build

Module resolver enforces visibility rules: providers are private to their declaring module unless listed in `exports`. Importing module A from module B does not auto-leak A's providers; if B wants to re-export A's provider, it must list it in B's `exports`. Controllers are always globally registered.

Boot-time validation:

- Detect duplicate provider tokens across the flattened graph; error with both declaring modules named.
- Detect cycles between modules and between providers.
- Verify every `@Inject(TOKEN)` resolves to a visible provider; error with the consumer and unresolved token.

No glob / auto-discovery. All imports are explicit.

## Acceptance criteria

- [ ] Private-by-default providers: importing module A in module B does not let B's providers inject A's non-exported services.
- [ ] `exports` lists make providers visible to importers.
- [ ] Re-exports through chains (A imports B, A exports B's token) are explicit and required for transitive visibility.
- [ ] Duplicate tokens across the flattened module graph throw at boot with both source modules in the message.
- [ ] Unresolved `@Inject(TOKEN)` references throw at boot with consumer + token info.
- [ ] No file-system / glob-based module loading anywhere in the boot path.
- [ ] Unit tests cover all visibility scenarios (private, exported, re-exported, leak attempt) and each validation error class.

## Blocked by

- [03: DI and dynamic modules](./03-di-and-dynamic-modules.md)
