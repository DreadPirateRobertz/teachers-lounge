# TeachersLounge TDD Specification

> This document is referenced during ALL PR reviews to verify TDD adherence.
> Non-compliance blocks merge.

---

## The TDD Cycle

Every feature follows this exact sequence.  Skipping steps is not permitted.

```
1. Write the failing test           ← proves the test actually tests something
2. Run — verify FAIL (not skip)     ← failing test is evidence the behaviour is missing
3. Write minimal implementation     ← only enough to make the test pass
4. Run — verify PASS                ← confirms the implementation is correct
5. Refactor (optional)              ← keep tests green throughout
6. Commit                           ← each passing test cycle is a commit
```

---

## Required Scenarios

Every exported function, method, and HTTP handler must have tests for:

| Category | Examples |
|----------|---------|
| Happy path | Valid input produces expected output |
| Invalid input | Missing field, wrong type, out-of-range value |
| Boundary values | 0, -1, max int, empty string, nil/null |
| Not found | Resource doesn't exist (404 equivalent) |
| Auth failures | Unauthenticated (401), unauthorised (403) |
| I/O failures | DB error, network timeout, Qdrant unavailable |
| Concurrent access | Race conditions on shared state (Go: `-race` flag) |
| Graceful degradation | Feature fails but request completes (non-fatal paths) |

---

## Language Standards

### Go

```go
// File: internal/handler/foo_test.go
// Run: go test ./... -race -count=1

func TestFooHandler_Returns200OnValidInput(t *testing.T) { ... }
func TestFooHandler_Returns400OnMissingField(t *testing.T) { ... }
func TestFooHandler_Returns401WhenUnauthenticated(t *testing.T) { ... }
func TestFooHandler_Returns500OnDBError(t *testing.T) { ... }
func TestFooHandler_GracefulDegradation_ContinuesOnCacheFailure(t *testing.T) { ... }
```

- Use `testify/require` for fatally failing assertions
- Use `testify/assert` for non-fatal assertions
- Integration tests use real DB (miniredis for Redis) — no mocking the DB layer
- Race detector always on in CI: `go test -race ./...`

### Python

```python
# File: tests/test_foo.py
# Run: pytest tests/ -v --tb=short

def test_foo_returns_result_on_valid_input(): ...
def test_foo_raises_value_error_on_empty_list(): ...
def test_foo_raises_http_401_when_token_missing(): ...
def test_foo_logs_warning_and_returns_none_on_db_error(): ...
```

- Use `pytest` — no `unittest` unless already established in file
- Use `pytest.raises` for expected exceptions
- Mock at the boundary: HTTP clients, external APIs, file I/O
- **Never mock the database in integration tests** — use the test DB

### TypeScript / React

```typescript
// File: __tests__/Foo.test.tsx
// Run: npm test

test('renders correctly with valid props', () => { ... })
test('shows error state when data is empty', () => { ... })
test('calls onError when fetch fails', () => { ... })
test('is accessible: has role and aria-label', () => { ... })
```

- Use `@testing-library/react` for component tests
- Use `jest.spyOn` for mocking — no manual mocks unless necessary
- Snapshot tests discouraged — prefer behavioural assertions

---

## Coverage Gate

| Requirement | Value |
|-------------|-------|
| Patch coverage (new code in PR) | ≥ 80% |
| Project coverage (overall) | ≥ 90% (soft — enforced in `codecov.yml`) |
| Excluded from coverage | `*_test.go`, `test_*.py`, `*.test.ts`, migrations, infra |

Codecov comments on every PR with patch coverage.  Below 80% = blocked.

---

## What Reviewers Check

When running `pr-test-analyzer` as part of the 5-agent review, verify:

1. Does every exported symbol have at least one test?
2. Is there at least one failure/error-case test per handler/function?
3. Are boundary values tested (0, empty, max)?
4. Are auth paths tested (401, 403)?
5. Are I/O error paths tested?
6. Does the test name describe the scenario, not the implementation?
7. Is the test isolated — does it clean up after itself?
8. Does the test fail first (TDD compliance) — reviewers can check git history?

---

## Anti-Patterns (auto-reject)

| Anti-pattern | Why |
|--------------|-----|
| Tests that always pass | No coverage of actual failure modes |
| Mocking the DB in integration tests | Masks schema/migration divergence |
| Single `test_it_works` test | Not TDD — describes implementation, not behaviour |
| Tests that test internals | Coupling to implementation, not contract |
| Tests that share mutable state | Flaky — order-dependent |
| Assertions without explanatory message | Hard to diagnose failures in CI |

---

*Referenced by all PR reviews. Last updated: 2026-04-12.*
