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
- Outlier + failure scenarios — **not optional** (empty input, boundary values,
  auth failure, network error, DB error, graceful degradation)

A test suite covering only the happy path will be **rejected in review**.

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

Every PR requires a **5-agent written code review** posted as a GitHub PR
comment before merge.  Approve/reject without a written review is insufficient.

### 5-Agent Review (required for every PR)

Post the combined review as a single comment with all five sections labelled:

| Agent | Checks |
|-------|--------|
| **code-reviewer** | Logic correctness, bug risk, security vulnerabilities |
| **silent-failure-hunter** | Error handling, catch blocks, silent failures, fallback |
| **type-design-analyzer** | Type invariants, encapsulation, usefulness |
| **comment-analyzer** | Comment accuracy, WHY vs WHAT, technical debt |
| **pr-test-analyzer** | Coverage completeness, missing edge cases, outlier tests |

PM (Petra) or the Refinery posts the review and is final arbiter on merge.

### Gate conditions — reject immediately if:
- No tests for new feature code
- No docstrings on exported symbols
- Codecov patch coverage < 80%
- Any CodeQL FAILURE in new code
- CI not green (all required checks must pass)
- No 5-agent review comment posted

### Process
1. Crew opens PR with filled-out body template (What / Why / How to test / Checklist).
2. PM or Refinery runs 5-agent review, posts comment.
3. Crew addresses review feedback.
4. PM merges — no force-pushes to `main`.
5. Squash merges preferred for clean history.

### Commit style
```
feat(scope): short description

Longer explanation if needed.

Closes tl-XXXX
Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
```
