## Summary

<!-- 2-3 bullets: what does this PR do at a high level? -->
-
-

## Bead

Closes `tl-XXXX`

## Type of Change

- [ ] New feature
- [ ] Bug fix
- [ ] Refactor / cleanup
- [ ] Infrastructure / config
- [ ] Documentation
- [ ] Test coverage

## Current vs New Behavior

**Before:** <!-- What happened before? What was broken, missing, or different? -->

**After:** <!-- What happens now? Be specific — endpoints, UI states, error messages, etc. -->

## Detail

<!-- Go deep. For features: data flow, schema changes, API contracts, UI states.
     For bugs: root cause, why it happened, why this fix is correct.
     For refactors: what changed structurally, what stayed the same.
     For docs: what was wrong/missing, what is now accurate. -->

## How to Test

<!-- Step-by-step a reviewer can follow to verify this works -->
1.
2.

## Checklist

**Code quality**
- [ ] All tests pass locally
- [ ] No lint errors (`npm run lint` / `ruff check` / `go vet`)
- [ ] Codecov patch coverage >= 90% on new code
- [ ] New functions/classes have docstrings (Google-style Python, JSDoc TS, godoc Go)
- [ ] No debug statements left in (console.log, print, fmt.Println)

**TDD**
- [ ] Tests written alongside or before implementation
- [ ] Happy-path test included
- [ ] Error/edge-case test included
- [ ] No new feature code without tests

**Security**
- [ ] No secrets, API keys, or `.env` files committed
- [ ] New env vars documented in `.env.example`
- [ ] User input validated at service boundaries
- [ ] No unsafe HTML injection patterns

**Compatibility**
- [ ] No breaking changes to existing API contracts (or documented below)
- [ ] DB migrations are backwards-compatible
- [ ] New services have `/healthz` endpoint
- [ ] Docker image builds successfully

## Breaking Changes

<!-- List breaking changes to APIs, env vars, or DB schema. "None" if not applicable. -->
None

## Self-Review

- [ ] I have read my own diff and confirmed it matches the intent above
