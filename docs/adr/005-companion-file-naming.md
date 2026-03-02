# ADR-005: Companion File Naming Convention

## Status
Accepted

## Context
tsgonest generates companion files alongside tsgo's JavaScript output. These files contain runtime validators (`validate`/`assert`), fast JSON serializers (`stringify`/`serialize`), and JSON Schema functions (`schema`) for each type.

## Decision
Each non-controller type gets a single companion file with the `.tsgonest.js` suffix (and `.tsgonest.d.ts` for types):

```
dist/
  user.dto.js                    # tsgo output
  user.dto.UserDto.tsgonest.js   # companion
  user.dto.UserDto.tsgonest.d.ts # companion types
```

The naming pattern is: `{sourceBaseName}.{TypeName}.tsgonest.{ext}`.

Controller classes (detected via `@Controller()` decorator) are intentionally skipped for companion generation — they are consumers of companion files, not sources of serializable types.

## Consequences
- One companion per type, not per source file — avoids monolithic companion files.
- The `.tsgonest.js` suffix is distinct from any user code.
- Controllers never get companions — this prevents circular import issues.
- File names are deterministic and predictable from the source structure.
