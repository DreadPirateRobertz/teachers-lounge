# Contributing to TeachersLounge

This document defines the quality bar for all code in this repository.
Every pull request is held to these standards. PRs missing required items
will be closed without review.

---

## TDD Standard

Write tests **alongside or before** implementation. No feature code ships
without tests.

### Minimum per exported function
- One happy-path test
- One error or edge-case test

### File placement
| Language | Test file |
|----------|-----------|
| Go | `<file>_test.go` in the same package |
| Python | `tests/test_<module>.py` |
| TypeScript/React | `<Component>.test.ts(x)` or `__tests__/<Component>.test.ts(x)` |

### Test runners
- Go: `go test ./...`
- Python: `pytest`
- TypeScript: `jest --passWithNoTests`

Coverage gate: **≥ 80% patch coverage** on every PR (Codecov enforces this).
PRs that drop coverage below 80% on new code are rejected.

### What to test
Write tests that document *behaviour*, not implementation details.

```go
// Good — tests the contract
func TestCreateUser_ReturnsErrOnDuplicateEmail(t *testing.T) { ... }

// Bad — tests internal state
func TestCreateUser_CallsSQLInsertOnce(t *testing.T) { ... }
```

Use real dependencies where practical. Mock at the boundary (HTTP, external
APIs, clock). Never mock the database in integration tests — we learned this
the hard way when mock tests passed but a prod migration failed.

---

## Logging

Log **sparingly and structuredly**. Never vomit logs.

### Rules
1. Use structured key-value fields, not interpolated strings.
2. Log at the **decision point**, not in every function.
3. Errors get logged once — at the point where context is lost (e.g. HTTP
   handler), not at every layer they pass through.
4. Include only fields that help diagnose the problem: `user_id`, `trace_id`,
   `duration_ms`, `error`. Not every variable in scope.

```go
// Good
slog.Error("payment failed", "user_id", userID, "error", err)

// Bad — vomit logging
log.Printf("entering CreateSubscription for user %s with plan %s and price %d...", ...)
```

```python
# Good
logger.error("rag_query_failed", extra={"user_id": user_id, "error": str(e)})

# Bad
print(f"DEBUG: got response {resp}, now processing, user is {user}...")
```

---

## Graceful Failure

Errors must be handled explicitly. Silent failures and panics are rejected in review.

### Rules

1. **Never swallow errors.** Every error must be logged or returned — not ignored.
2. **Return errors to the caller** from library/service code. Log at the boundary (HTTP handler, worker loop), not at every layer.
3. **Fail loudly at startup** for missing config. If a required env var is absent, the process should exit with a clear message — not limp along with zero-values.
4. **Partial failures need a decision.** If one item in a batch fails, decide explicitly: abort, skip + log, or retry. Never silently drop.
5. **No panic in service code.** Recover panics at the top-level HTTP handler only (for crash isolation). Never panic in business logic.

```go
// Good — explicit, logged at boundary
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
    user, err := h.store.CreateUser(r.Context(), params)
    if err != nil {
        slog.Error("create_user_failed", "error", err)
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }
    ...
}

// Bad — silent drop
user, _ := h.store.CreateUser(r.Context(), params)
```

```python
# Good — explicit failure with context
try:
    result = qdrant_client.upsert(collection_name=collection, points=points)
except Exception as e:
    logger.error("qdrant_upsert_failed", extra={"collection": collection, "error": str(e)})
    raise

# Bad — swallowed error
try:
    qdrant_client.upsert(...)
except Exception:
    pass
```

### Tests must cover failure paths

Every exported function with error paths needs at least one test that triggers
the error path and asserts the correct error is returned or logged.

```go
func TestCreateUser_ReturnsErrOnDuplicateEmail(t *testing.T) { ... }
func TestCreateUser_LogsAndReturns500OnStoreFailure(t *testing.T) { ... }
```

---

## Comments

Write **WHY**, never WHAT or HOW.

The code already shows what it does. A comment that restates the code adds
noise and rots. A comment that explains *why* this non-obvious choice was
made saves the next engineer hours.

```go
// Good — explains non-obvious intent
// Skip the cache here: subscription status must reflect live DB state
// because a billing webhook may have just changed it.
sub, err := store.GetSubscriptionByUserID(ctx, userID)

// Bad — restates the code
// Get subscription by user ID
sub, err := store.GetSubscriptionByUserID(ctx, userID)
```

```typescript
// Good
// Defer MoleculeViewer import: Three.js crashes in Node (SSR).
// This must remain dynamic with ssr: false.
const MoleculeViewer = dynamic(() => import('./MoleculeViewerCanvas'), { ssr: false })

// Bad
// Import molecule viewer component
const MoleculeViewer = dynamic(...)
```

---

## Docstrings / Godoc

Every **exported** symbol needs a doc comment.

### Go (godoc style)
```go
// CreateUser inserts a new user record and initialises their learning and
// gaming profiles. Returns ErrDuplicateEmail if the email is already taken.
func (s *Store) CreateUser(ctx context.Context, p CreateUserParams) (*models.User, error)
```

### Python (Google style)
```python
def classify_style(answers: list[str]) -> LearningStyle:
    """Classify learning style from assessment answers.

    Args:
        answers: Chosen keys ('A' or 'B') from each assessment question.

    Returns:
        LearningStyle with dimension scores in [-1.0, 1.0].
    """
```

### TypeScript (JSDoc)
```typescript
/**
 * Renders a 3-D molecule by SMILES key using Three.js / R3F.
 * Must be used inside a Canvas context.
 */
export function MoleculeViewer({ molecule }: { molecule: string }): JSX.Element
```

---

## PR Checklist

Before opening a PR:

- [ ] Tests written (TDD) — list test files added/modified in the PR body
- [ ] Coverage ≥ 80% on new code (Codecov will comment)
- [ ] Docstrings on all new exported symbols
- [ ] `go vet ./...` / `ruff check` / `next lint` clean
- [ ] `npm run format:check` or `gofmt` passes
- [ ] No secrets or `.env` files committed
- [ ] Self-review: read your own diff before opening

### PR body template
```
## What
Brief description of the change.

## Why
Bead ID: tl-XXXX — link to spec section if relevant.

## How to test
Steps to verify the change works.

## Checklist
- [ ] Tests written (TDD) — test files: <list>
- [ ] Coverage ≥ 80% on new code
- [ ] Docstrings on all new exported functions/classes
- [ ] Lint clean
- [ ] No secrets committed
```

### Branch naming
```
feat/<bead-id>-short-description
fix/<bead-id>-short-description
chore/<bead-id>-short-description
```

---

## Mobile / Device Compatibility

For any PR touching frontend UI components, run the device compatibility
checklist before requesting review:

### Layout
- [ ] Safe area insets applied (notch, Dynamic Island, home indicator)
- [ ] Keyboard avoidance: content not hidden by soft keyboard on form screens
- [ ] Content not obscured by status bar or nav bar
- [ ] Landscape handled or explicitly locked
- [ ] Small screen (375 px / SE 1st gen) renders without overflow

### Touch
- [ ] Touch targets ≥ 44 × 44 pt
- [ ] No tap-through gaps between adjacent pressables

### Platform
- [ ] iOS Safari: `position: fixed` and scroll behaviour correct
- [ ] Android Chrome: back gesture does not break nav state
- [ ] Dark mode / light mode both correct

### Network
- [ ] Offline state shows error or empty state (not blank screen)
- [ ] Request timeout handled (not infinite spinner)

### Accessibility
- [ ] Screen reader labels on all interactive elements
- [ ] Minimum contrast ratio 4.5 : 1 for text
- [ ] Logical focus order

---

## Review Process

1. Every PR needs **1 review minimum** (PM or another crew member).
2. Reviewer checks: TDD compliance, docstring coverage, correctness, security.
3. **Reject immediately** if: no tests, no docstrings, Codecov drops below
   80% patch coverage.
4. No force-pushes to `main` — PRs only.
5. Squash merges preferred for clean history.
6. Crew members **cannot self-merge** — PM or Refinery is final arbiter.

### Commit style
```
feat(scope): short description

Longer explanation if needed.

Closes tl-XXXX
Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
```
