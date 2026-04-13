# TeachersLounge Feature Roadmap — Remaining Work Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close Phases 2 and 4 (the two active construction zones), complete the remaining Phase 5/8 hardening items, and reach production-ready state.

**Architecture:** Six microservices (tutoring/gaming/user/ingestion/search/notification) on GKE, with Qdrant vector DB, Postgres (Cloud SQL), Redis (Memorystore), and LiteLLM AI gateway. Frontend is Next.js with Three.js boss battles. All services containerized, GitOps via GitHub Actions.

**Tech Stack:** Go (gaming/user/notification), Python/FastAPI (tutoring/ingestion/search/analytics), Next.js/React/Three.js (frontend), Qdrant, Postgres, Redis, LiteLLM, Helm, GKE.

**Current branch context:** Each task maps to an existing bead ID. Crew assignments shown. Owner is responsible for TDD compliance per CLAUDE.md.

---

## Phase Status Snapshot

| Phase | Status | Blocking Items |
|-------|--------|----------------|
| 1 — Foundation | ✅ Complete | — |
| 2 — RAG Core | 🔄 ~70% | PDF pipeline, Qdrant prod, upload UI |
| 3 — Gaming Layer | ✅ ~90% | — |
| 4 — Boss Battles | 🔄 ~55% | Backend CI, frontend state machine |
| 5 — Adaptive Learning | ✅ ~90% | PR #213 (context window) |
| 6 — Full Ingestion | ✅ ~80% | Tied to Phase 2 PDF pipeline |
| 7 — Analytics + Privacy | ✅ Complete | — |
| 8 — Polish + Scale | ✅ ~85% | OTel upgrade, JWT validation |

---

## TRACK A: Phase 2 — RAG Core Completion

### Task A1: Boss Battle Backend CI Fix (hq-cv-eebuu / carn)

**Bead:** `hq-cv-iguk6` unblocked by this fix  
**Owner:** carn  
**Branch:** `feat/tl-7ot-boss-battle-backend`

**Files:**
- Modify: `.github/workflows/ci.yml` (setup-go version, golangci-lint-action version)
- Modify: `services/user-service/internal/handlers/auth_test.go:340,356,386,478`
- Modify: `services/user-service/internal/cache/redis.go:97`
- Modify: `services/user-service/internal/auth/jwt.go:17-20`
- Modify: `services/user-service/internal/store/store.go:54`

- [ ] **Step 1: Fix errcheck violations in auth_test.go**

  In `auth_test.go:340`, wrap unchecked call:
  ```go
  err := s.CreateSubscription(context.Background(), store.CreateSubscriptionParams{...})
  require.NoError(t, err)
  ```
  For `json.NewDecoder(w.Body).Decode(&resp)` at lines 356, 386, 478:
  ```go
  require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
  ```

- [ ] **Step 2: Fix staticcheck — deprecated SetNX in cache/redis.go:97**

  Replace:
  ```go
  return c.rdb.SetNX(ctx, key, value, ttl).Result()
  ```
  With:
  ```go
  return c.rdb.Set(ctx, key, value, ttl).SetNX().Result()
  ```

- [ ] **Step 3: Remove unused constants from auth/jwt.go:17-20**

  Delete the four unused constants (`claimsUserID`, `claimsEmail`, `claimsAccountType`, `claimsSubStatus`) or wire them into the JWT claims struct if they were meant to be used. Check usages first:
  ```bash
  grep -rn "claimsUserID\|claimsEmail\|claimsAccountType\|claimsSubStatus" services/user-service/
  ```
  If no usages: delete. If there are usages elsewhere: keep and wire up.

- [ ] **Step 4: Remove or use execWithRLS in store.go:54**

  ```bash
  grep -rn "execWithRLS" services/user-service/
  ```
  If unused: delete the function. If it was intended: add a usage or document why it's retained with `//nolint:unused`.

- [ ] **Step 5: Run lint locally to verify clean**

  ```bash
  cd services/user-service
  golangci-lint run ./...
  ```
  Expected: no issues reported.

- [ ] **Step 6: Run tests**

  ```bash
  go test ./... -v 2>&1 | tail -20
  ```
  Expected: all pass.

- [ ] **Step 7: Commit and push**

  ```bash
  git add services/user-service/ .github/workflows/
  git commit -m "fix(ci): bump golangci-lint to v2.5.0 + fix 13 lint issues in user-service

  - setup-go 1.23→1.25 to match go.mod target
  - golangci-lint-action v6→v8, version v2.5.0 (Go 1.25 compatible)
  - errcheck: wrap CreateSubscription + Decode calls with require.NoError
  - staticcheck: replace deprecated SetNX with Set+NX option
  - unused: remove dead claimsUserID/Email/AccountType/SubStatus consts
  - unused: remove execWithRLS if unreferenced

  Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
  git push
  ```

- [ ] **Step 8: Watch CI pass, report to petra**

---

### Task A2: Material Upload UI — Codecov Fix (hq-cv-eebuu / alai)

**Bead:** `hq-cv-eebuu`  
**Owner:** alai  
**Branch:** `feat/hq-cv-eebuu-material-upload-ui`  
**PR:** #145 (Codecov patch check failing)

**Files:**
- Check: `https://app.codecov.io/gh/DreadPirateRobertz/teachers-lounge/pull/145` for uncovered lines
- Test: whichever component test files are missing coverage

- [ ] **Step 1: Identify uncovered lines**

  ```bash
  # Check Codecov report for which lines are uncovered
  gh pr checks 145 --repo DreadPirateRobertz/teachers-lounge
  # Open the codecov/patch URL from the output
  ```

- [ ] **Step 2: For each uncovered component, write the missing test**

  Pattern for Next.js components (place in `__tests__/<Component>.test.tsx`):
  ```typescript
  import { render, screen, fireEvent, waitFor } from '@testing-library/react'
  import { MaterialUpload } from '../components/MaterialUpload'

  describe('MaterialUpload', () => {
    it('shows processing status after file selected', async () => {
      render(<MaterialUpload onUpload={jest.fn()} />)
      const input = screen.getByLabelText(/upload/i)
      fireEvent.change(input, { target: { files: [new File(['pdf'], 'test.pdf')] } })
      await waitFor(() => expect(screen.getByText(/processing/i)).toBeInTheDocument())
    })

    it('shows error state on upload failure', async () => {
      const onUpload = jest.fn().mockRejectedValue(new Error('upload failed'))
      render(<MaterialUpload onUpload={onUpload} />)
      // trigger upload, assert error message shown
    })
  })
  ```

- [ ] **Step 3: Run tests to verify coverage**

  ```bash
  cd frontend && npm test -- --coverage --collectCoverageFrom='app/materials/**'
  ```
  Expected: patch coverage ≥80%.

- [ ] **Step 4: Commit and push**

  ```bash
  git add frontend/__tests__/
  git commit -m "test(hq-cv-eebuu): add missing coverage for material upload components

  Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
  git push
  ```

- [ ] **Step 5: Confirm Codecov patch check passes on PR #145**

---

### Task A3: PDF Processing Pipeline (hq-cv-iguk6 / dink)

**Bead:** `hq-cv-iguk6`  
**Owner:** dink  
**Branch:** `feat/hq-cv-iguk6-pdf-pipeline`

**Files:**
- Modify: `services/ingestion/app/processors/pdf.py` — unstructured.io processing
- Create: `services/ingestion/app/chunking/hierarchical.py` — hierarchical chunker
- Modify: `services/ingestion/app/embeddings.py` — OpenAI embedding generation
- Modify: `services/ingestion/app/qdrant_client.py` — write to curriculum collection
- Modify: `services/ingestion/app/database.py` — write to Postgres chunk registry
- Test: `services/ingestion/tests/test_pdf_pipeline.py`
- Test: `services/ingestion/tests/test_hierarchical_chunking.py`

- [ ] **Step 1: Write failing test for hierarchical chunking**

  ```python
  # tests/test_hierarchical_chunking.py
  def test_chunks_respect_section_boundaries():
      text = "# Chapter 1\nContent A\n## Section 1.1\nContent B\n# Chapter 2\nContent C"
      chunks = hierarchical_chunk(text, max_chunk_tokens=50)
      # Chunks should not span chapter boundaries
      assert not any("Chapter 1" in c and "Chapter 2" in c for c in chunks)

  def test_chunk_metadata_includes_hierarchy():
      text = "# Chapter 1\n## Section 1.1\nsome content"
      chunks = hierarchical_chunk(text, max_chunk_tokens=100)
      assert chunks[0].metadata["section"] == "Section 1.1"
      assert chunks[0].metadata["chapter"] == "Chapter 1"
  ```

- [ ] **Step 2: Run failing tests to confirm they fail**

  ```bash
  cd services/ingestion && pytest tests/test_hierarchical_chunking.py -v
  ```

- [ ] **Step 3: Implement hierarchical_chunk in chunking/hierarchical.py**

  ```python
  """Hierarchical chunking — splits documents respecting section structure."""
  from __future__ import annotations
  from dataclasses import dataclass, field
  from typing import Optional
  import re

  @dataclass
  class Chunk:
      """A document chunk with hierarchy metadata."""
      content: str
      metadata: dict = field(default_factory=dict)

  def hierarchical_chunk(text: str, max_chunk_tokens: int = 512) -> list[Chunk]:
      """Split text into chunks respecting Markdown section boundaries.

      Args:
          text: Raw document text (Markdown or plain).
          max_chunk_tokens: Approximate max tokens per chunk (4 chars/token).

      Returns:
          List of Chunk objects with chapter/section metadata.
      """
      max_chars = max_chunk_tokens * 4
      # Split on h1/h2 boundaries
      sections = re.split(r'(?=^#{1,2} )', text, flags=re.MULTILINE)
      chunks = []
      current_chapter = ""
      current_section = ""
      for section in sections:
          header_match = re.match(r'^(#{1,2}) (.+)', section)
          if header_match:
              level, title = len(header_match.group(1)), header_match.group(2).strip()
              if level == 1:
                  current_chapter = title
                  current_section = ""
              else:
                  current_section = title
          # Split oversized sections further
          content = section.strip()
          while len(content) > max_chars:
              chunks.append(Chunk(
                  content=content[:max_chars],
                  metadata={"chapter": current_chapter, "section": current_section}
              ))
              content = content[max_chars:]
          if content:
              chunks.append(Chunk(
                  content=content,
                  metadata={"chapter": current_chapter, "section": current_section}
              ))
      return chunks
  ```

- [ ] **Step 4: Run tests — verify passing**

  ```bash
  pytest tests/test_hierarchical_chunking.py -v
  ```

- [ ] **Step 5: Write failing test for full PDF pipeline**

  ```python
  # tests/test_pdf_pipeline.py
  from unittest.mock import AsyncMock, patch, MagicMock
  import pytest

  @pytest.mark.asyncio
  async def test_pdf_pipeline_writes_chunks_to_qdrant():
      pdf_bytes = open("tests/fixtures/sample.pdf", "rb").read()
      mock_qdrant = AsyncMock()
      mock_openai = AsyncMock()
      mock_openai.embeddings.create = AsyncMock(return_value=MagicMock(
          data=[MagicMock(embedding=[0.1] * 1536)]
      ))
      with patch("app.qdrant_client.get_client", return_value=mock_qdrant):
          with patch("app.embeddings.get_openai_client", return_value=mock_openai):
              result = await process_pdf(pdf_bytes, course_id="course-123")
      assert result.chunk_count > 0
      assert mock_qdrant.upsert.called
  ```

- [ ] **Step 6: Wire up process_pdf in processors/pdf.py**

  ```python
  """PDF processing pipeline — unstructured.io → chunk → embed → Qdrant + Postgres."""
  from __future__ import annotations
  from dataclasses import dataclass
  from unstructured.partition.pdf import partition_pdf
  from .chunking.hierarchical import hierarchical_chunk
  from ..embeddings import generate_embeddings
  from ..qdrant_client import upsert_chunks
  from ..database import register_chunks

  @dataclass
  class PipelineResult:
      """Result of a PDF processing run."""
      chunk_count: int
      material_id: str

  async def process_pdf(pdf_bytes: bytes, course_id: str) -> PipelineResult:
      """Process a PDF through the full ingestion pipeline.

      Args:
          pdf_bytes: Raw PDF file contents.
          course_id: Course this material belongs to.

      Returns:
          PipelineResult with chunk count and generated material ID.
      """
      import tempfile, os, uuid
      material_id = str(uuid.uuid4())
      with tempfile.NamedTemporaryFile(suffix=".pdf", delete=False) as f:
          f.write(pdf_bytes)
          tmp_path = f.name
      try:
          elements = partition_pdf(filename=tmp_path, strategy="hi_res")
          text = "\n".join(str(e) for e in elements)
      finally:
          os.unlink(tmp_path)
      chunks = hierarchical_chunk(text)
      embeddings = await generate_embeddings([c.content for c in chunks])
      await upsert_chunks(chunks, embeddings, course_id=course_id, material_id=material_id)
      await register_chunks(chunks, course_id=course_id, material_id=material_id)
      return PipelineResult(chunk_count=len(chunks), material_id=material_id)
  ```

- [ ] **Step 7: Run full test suite**

  ```bash
  cd services/ingestion && pytest tests/ -v --tb=short
  ```

- [ ] **Step 8: Commit**

  ```bash
  git add services/ingestion/
  git commit -m "feat(hq-cv-iguk6): PDF processing pipeline — hierarchical chunking + embedding + Qdrant write

  Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
  git push
  ```

---

### Task A4: Qdrant Production Helm Chart (hq-cv-kqszc / bean)

**Bead:** `hq-cv-kqszc`  
**Owner:** bean  
**Branch:** `feat/hq-cv-kqszc-qdrant-prod`

**Files:**
- Create: `helm/qdrant/values.yaml` — 3-replica config, HNSW + scalar quantization
- Create: `helm/qdrant/values-prod.yaml` — production overrides
- Create: `helm/qdrant/templates/snapshot-cronjob.yaml` — GCS nightly snapshots
- Create: `helm/qdrant/Chart.yaml`
- Test: `helm/qdrant/tests/test_helm_render.sh` — helm template smoke test

- [ ] **Step 1: Write Chart.yaml**

  ```yaml
  # helm/qdrant/Chart.yaml
  apiVersion: v2
  name: qdrant
  description: Qdrant vector database — 3-replica production deployment
  type: application
  version: 0.1.0
  appVersion: "1.8.0"
  dependencies:
    - name: qdrant
      version: "0.8.4"
      repository: "https://qdrant.github.io/qdrant-helm"
  ```

- [ ] **Step 2: Write values.yaml (3-replica, HNSW, scalar quantization)**

  ```yaml
  # helm/qdrant/values.yaml
  qdrant:
    replicaCount: 3

    config:
      cluster:
        enabled: true
        p2p:
          port: 6335

    persistence:
      size: 50Gi
      storageClass: standard-rwo

    resources:
      requests:
        memory: "4Gi"
        cpu: "1"
      limits:
        memory: "8Gi"
        cpu: "2"

    # HNSW index config (set at collection creation time via API, not Helm)
    # m: 16, ef_construct: 100 — good balance of speed/recall for curriculum vectors

  snapshotCronJob:
    enabled: true
    schedule: "0 2 * * *"   # 2am UTC nightly
    gcsBucket: "teachers-lounge-qdrant-snapshots"
    gcsPath: "nightly"
    image: curlimages/curl:8.7.1
  ```

- [ ] **Step 3: Write snapshot CronJob template**

  ```yaml
  # helm/qdrant/templates/snapshot-cronjob.yaml
  {{- if .Values.snapshotCronJob.enabled }}
  apiVersion: batch/v1
  kind: CronJob
  metadata:
    name: {{ include "qdrant.fullname" . }}-snapshot
    labels: {{ include "qdrant.labels" . | nindent 4 }}
  spec:
    schedule: {{ .Values.snapshotCronJob.schedule | quote }}
    concurrencyPolicy: Forbid
    jobTemplate:
      spec:
        template:
          spec:
            restartPolicy: OnFailure
            containers:
              - name: snapshot
                image: {{ .Values.snapshotCronJob.image }}
                command:
                  - /bin/sh
                  - -c
                  - |
                    set -e
                    DATE=$(date +%Y%m%d)
                    # Trigger snapshot on all collections
                    curl -sf -X POST "http://{{ include "qdrant.fullname" . }}:6333/collections/curriculum/snapshots"
                    curl -sf -X POST "http://{{ include "qdrant.fullname" . }}:6333/collections/insights/snapshots"
                    echo "Snapshots created for $DATE"
  {{- end }}
  ```

- [ ] **Step 4: Run helm lint**

  ```bash
  helm dependency update helm/qdrant/
  helm lint helm/qdrant/
  ```
  Expected: `1 chart(s) linted, 0 chart(s) failed`.

- [ ] **Step 5: Run helm template smoke test**

  ```bash
  helm template test-release helm/qdrant/ | grep -E "kind:|name:" | head -20
  ```
  Expected: shows StatefulSet (3 replicas), Service, CronJob.

- [ ] **Step 6: Commit**

  ```bash
  git add helm/qdrant/
  git commit -m "feat(hq-cv-kqszc): Qdrant 3-replica Helm chart + GCS nightly snapshot CronJob

  Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
  git push
  ```

---

## TRACK B: Phase 4 — Boss Battles Completion

### Task B1: Boss Battle Frontend — State Machine (tl-2l7 / shen)

**Bead:** `tl-2l7`  
**Owner:** shen  
**Depends on:** PR #211 merged  
**Branch:** `feat/tl-2l7-battle-state-machine`

**Files:**
- Create: `frontend/app/boss-battle/[id]/state-machine.ts` — XState or reducer-based FSM
- Modify: `frontend/app/boss-battle/[id]/page.tsx` — wire state machine
- Create: `frontend/components/battle/HealthBar.tsx` — animated HP bar
- Create: `frontend/components/battle/PowerUpPanel.tsx` — cooldown timers, activate
- Test: `frontend/__tests__/battle/state-machine.test.ts`
- Test: `frontend/__tests__/battle/HealthBar.test.tsx`

- [ ] **Step 1: Write failing test for state machine phases**

  ```typescript
  // __tests__/battle/state-machine.test.ts
  import { createBattleReducer, BattleState } from '../../app/boss-battle/[id]/state-machine'

  const reducer = createBattleReducer()

  test('transitions intro → question on START', () => {
    const state: BattleState = { phase: 'intro', playerHP: 100, bossHP: 200, round: 1 }
    const next = reducer(state, { type: 'START' })
    expect(next.phase).toBe('question')
  })

  test('transitions question → attack on ANSWER_CORRECT', () => {
    const state: BattleState = { phase: 'question', playerHP: 100, bossHP: 200, round: 1 }
    const next = reducer(state, { type: 'ANSWER_CORRECT', damage: 30 })
    expect(next.phase).toBe('attack')
    expect(next.bossHP).toBe(170)
  })

  test('transitions to defeat when playerHP reaches 0', () => {
    const state: BattleState = { phase: 'resolve', playerHP: 10, bossHP: 50, round: 2 }
    const next = reducer(state, { type: 'BOSS_ATTACKS', damage: 15 })
    expect(next.phase).toBe('defeat')
    expect(next.playerHP).toBe(0)
  })

  test('transitions to victory when bossHP reaches 0', () => {
    const state: BattleState = { phase: 'attack', playerHP: 80, bossHP: 20, round: 3 }
    const next = reducer(state, { type: 'DAMAGE_APPLIED', damage: 25 })
    expect(next.phase).toBe('victory')
    expect(next.bossHP).toBe(0)
  })
  ```

- [ ] **Step 2: Run failing tests**

  ```bash
  cd frontend && npm test -- __tests__/battle/state-machine.test.ts
  ```

- [ ] **Step 3: Implement state machine**

  ```typescript
  // app/boss-battle/[id]/state-machine.ts
  export type BattlePhase = 'intro' | 'question' | 'attack' | 'resolve' | 'victory' | 'defeat'

  export interface BattleState {
    phase: BattlePhase
    playerHP: number
    bossHP: number
    round: number
    activePowerUp?: string
  }

  export type BattleAction =
    | { type: 'START' }
    | { type: 'ANSWER_CORRECT'; damage: number }
    | { type: 'ANSWER_WRONG' }
    | { type: 'DAMAGE_APPLIED'; damage: number }
    | { type: 'BOSS_ATTACKS'; damage: number }
    | { type: 'RESOLVE_ROUND' }
    | { type: 'ACTIVATE_POWER_UP'; powerUp: string }

  export function createBattleReducer() {
    return function battleReducer(state: BattleState, action: BattleAction): BattleState {
      switch (action.type) {
        case 'START':
          return { ...state, phase: 'question' }
        case 'ANSWER_CORRECT':
          return { ...state, phase: 'attack', bossHP: Math.max(0, state.bossHP - action.damage) }
        case 'ANSWER_WRONG':
          return { ...state, phase: 'resolve' }
        case 'DAMAGE_APPLIED': {
          const newBossHP = Math.max(0, state.bossHP - action.damage)
          return { ...state, bossHP: newBossHP, phase: newBossHP === 0 ? 'victory' : 'resolve' }
        }
        case 'BOSS_ATTACKS': {
          const newPlayerHP = Math.max(0, state.playerHP - action.damage)
          return { ...state, playerHP: newPlayerHP, phase: newPlayerHP === 0 ? 'defeat' : 'question', round: state.round + 1 }
        }
        case 'RESOLVE_ROUND':
          return { ...state, phase: 'question', round: state.round + 1 }
        case 'ACTIVATE_POWER_UP':
          return { ...state, activePowerUp: action.powerUp }
        default:
          return state
      }
    }
  }
  ```

- [ ] **Step 4: Run tests — verify all pass**

  ```bash
  npm test -- __tests__/battle/state-machine.test.ts
  ```

- [ ] **Step 5: Write HealthBar component test**

  ```typescript
  // __tests__/battle/HealthBar.test.tsx
  import { render, screen } from '@testing-library/react'
  import { HealthBar } from '../../components/battle/HealthBar'

  test('renders correct percentage', () => {
    render(<HealthBar current={70} max={100} label="Player" />)
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '70')
  })

  test('shows critical styling below 25%', () => {
    const { container } = render(<HealthBar current={20} max={100} label="Player" />)
    expect(container.firstChild).toHaveClass('critical')
  })
  ```

- [ ] **Step 6: Implement HealthBar component**

  ```typescript
  // components/battle/HealthBar.tsx
  /** Animated health bar for boss battle UI. */
  interface HealthBarProps {
    current: number
    max: number
    label: string
  }

  export function HealthBar({ current, max, label }: HealthBarProps) {
    const pct = Math.round((current / max) * 100)
    const isCritical = pct < 25
    return (
      <div className={`health-bar ${isCritical ? 'critical' : ''}`}>
        <span className="health-bar__label">{label}: {current}/{max}</span>
        <div
          role="progressbar"
          aria-valuenow={current}
          aria-valuemin={0}
          aria-valuemax={max}
          className="health-bar__fill"
          style={{ width: `${pct}%` }}
        />
      </div>
    )
  }
  ```

- [ ] **Step 7: Run full frontend test suite**

  ```bash
  npm test -- --passWithNoTests
  ```

- [ ] **Step 8: Commit**

  ```bash
  git add frontend/
  git commit -m "feat(tl-2l7): boss battle state machine + HealthBar + PowerUpPanel

  Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
  git push
  ```

---

### Task B2: Boss Battle Frontend — Three.js Polish + WebSocket (tl-dye / shen)

**Bead:** `tl-dye`  
**Owner:** shen  
**Depends on:** Task B1 merged, PR #211 merged  
**Branch:** `feat/tl-dye-boss-battle-frontend`

**Files:**
- Modify: `frontend/app/boss-battle/[id]/page.tsx` — wire WebSocket, loot reveal
- Modify: `frontend/components/battle/BossScene.tsx` — Three.js hit reactions
- Create: `frontend/components/battle/LootReveal.tsx` — chest open → item reveal
- Create: `frontend/hooks/useBattleSocket.ts` — WebSocket connection + reconnect
- Test: `frontend/__tests__/battle/useBattleSocket.test.ts`
- Test: `frontend/__tests__/battle/LootReveal.test.tsx`

- [ ] **Step 1: Write failing test for useBattleSocket**

  ```typescript
  // __tests__/battle/useBattleSocket.test.ts
  import { renderHook, act } from '@testing-library/react'
  import { useBattleSocket } from '../../hooks/useBattleSocket'

  test('connects and receives state updates', async () => {
    const mockWs = { onmessage: null, onclose: null, close: jest.fn() }
    global.WebSocket = jest.fn(() => mockWs) as any

    const { result } = renderHook(() => useBattleSocket('session-123'))
    act(() => {
      mockWs.onmessage?.({ data: JSON.stringify({ bossHP: 180, playerHP: 95 }) } as any)
    })
    expect(result.current.battleState?.bossHP).toBe(180)
  })

  test('reconnects on close', async () => {
    const mockWs = { onmessage: null, onclose: null, close: jest.fn() }
    global.WebSocket = jest.fn(() => mockWs) as any
    jest.useFakeTimers()

    renderHook(() => useBattleSocket('session-123'))
    act(() => { mockWs.onclose?.({} as any) })
    jest.advanceTimersByTime(2000)
    expect(global.WebSocket).toHaveBeenCalledTimes(2)
    jest.useRealTimers()
  })
  ```

- [ ] **Step 2: Implement useBattleSocket hook**

  ```typescript
  // hooks/useBattleSocket.ts
  import { useEffect, useRef, useState } from 'react'

  interface BattleUpdate { bossHP: number; playerHP: number; event?: string }

  /** WebSocket hook for real-time boss battle state sync with auto-reconnect. */
  export function useBattleSocket(sessionId: string) {
    const [battleState, setBattleState] = useState<BattleUpdate | null>(null)
    const wsRef = useRef<WebSocket | null>(null)
    const reconnectTimer = useRef<ReturnType<typeof setTimeout>>()

    function connect() {
      const ws = new WebSocket(`/ws/gaming/boss/${sessionId}`)
      ws.onmessage = (e) => setBattleState(JSON.parse(e.data))
      ws.onclose = () => {
        reconnectTimer.current = setTimeout(connect, 2000)
      }
      wsRef.current = ws
    }

    useEffect(() => {
      connect()
      return () => {
        clearTimeout(reconnectTimer.current)
        wsRef.current?.close()
      }
    }, [sessionId])

    return { battleState }
  }
  ```

- [ ] **Step 3: Run WebSocket tests**

  ```bash
  npm test -- __tests__/battle/useBattleSocket.test.ts
  ```

- [ ] **Step 4: Write LootReveal test**

  ```typescript
  // __tests__/battle/LootReveal.test.tsx
  import { render, screen } from '@testing-library/react'
  import { LootReveal } from '../../components/battle/LootReveal'

  test('shows loot items after chest opens', async () => {
    const loot = [{ name: 'Victory Badge', type: 'badge' }]
    render(<LootReveal loot={loot} onContinue={jest.fn()} />)
    expect(await screen.findByText('Victory Badge')).toBeInTheDocument()
  })

  test('calls onContinue when continue button clicked', async () => {
    const onContinue = jest.fn()
    render(<LootReveal loot={[]} onContinue={onContinue} />)
    const btn = await screen.findByRole('button', { name: /continue/i })
    btn.click()
    expect(onContinue).toHaveBeenCalled()
  })
  ```

- [ ] **Step 5: Implement LootReveal component**

  ```typescript
  // components/battle/LootReveal.tsx
  'use client'
  import { useEffect, useState } from 'react'

  interface LootItem { name: string; type: 'badge' | 'gem' | 'cosmetic' }

  /** Post-battle loot reveal — chest open animation → item display. */
  export function LootReveal({ loot, onContinue }: { loot: LootItem[]; onContinue: () => void }) {
    const [open, setOpen] = useState(false)
    useEffect(() => { setTimeout(() => setOpen(true), 600) }, [])
    return (
      <div className="loot-reveal">
        <div className={`chest ${open ? 'chest--open' : ''}`} aria-label="loot chest" />
        {open && (
          <ul className="loot-reveal__items">
            {loot.map((item) => (
              <li key={item.name} className={`loot-item loot-item--${item.type}`}>{item.name}</li>
            ))}
          </ul>
        )}
        {open && <button onClick={onContinue}>Continue</button>}
      </div>
    )
  }
  ```

- [ ] **Step 6: Run full frontend test suite + coverage**

  ```bash
  npm test -- --coverage 2>&1 | tail -20
  ```
  Expected: patch coverage ≥80%.

- [ ] **Step 7: Commit**

  ```bash
  git add frontend/
  git commit -m "feat(tl-dye): Three.js boss battle polish — WebSocket sync, loot reveal, hit reactions

  Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
  git push
  ```

---

### Task B3: Boss Progression Map (tl-7wv / shen)

**Bead:** `tl-7wv`  
**Owner:** shen  
**Depends on:** Task B1, B2 merged  
**Branch:** `feat/tl-7wv-boss-progression-map`

**Files:**
- Create: `frontend/app/progression/page.tsx` — visual trail UI
- Create: `frontend/components/progression/BossNode.tsx` — defeated/current/locked states
- Create: `frontend/components/progression/ProgressionMap.tsx` — SVG trail connecting nodes
- Test: `frontend/__tests__/progression/ProgressionMap.test.tsx`

- [ ] **Step 1: Write failing test**

  ```typescript
  import { render, screen } from '@testing-library/react'
  import { ProgressionMap } from '../../components/progression/ProgressionMap'

  const bosses = [
    { id: '1', name: 'THE ATOM', status: 'defeated', masteryPct: 85 },
    { id: '2', name: 'THE BONDER', status: 'current', masteryPct: 42 },
    { id: '3', name: 'NAME LORD', status: 'locked', masteryPct: 0 },
  ]

  test('renders all boss nodes', () => {
    render(<ProgressionMap bosses={bosses} />)
    expect(screen.getByText('THE ATOM')).toBeInTheDocument()
    expect(screen.getByText('THE BONDER')).toBeInTheDocument()
    expect(screen.getByText('NAME LORD')).toBeInTheDocument()
  })

  test('shows unlock threshold on locked bosses', () => {
    render(<ProgressionMap bosses={bosses} unlockThreshold={60} />)
    expect(screen.getByText(/60%/)).toBeInTheDocument()
  })
  ```

- [ ] **Step 2: Implement ProgressionMap and BossNode**

  ```typescript
  // components/progression/BossNode.tsx
  interface BossNodeProps {
    name: string
    status: 'defeated' | 'current' | 'locked'
    masteryPct: number
    unlockThreshold?: number
  }

  /** Single boss node in the progression trail. */
  export function BossNode({ name, status, masteryPct, unlockThreshold = 60 }: BossNodeProps) {
    return (
      <div className={`boss-node boss-node--${status}`}>
        <span className="boss-node__name">{name}</span>
        {status === 'locked' && (
          <span className="boss-node__unlock">Need {unlockThreshold}% mastery</span>
        )}
        {status !== 'locked' && (
          <span className="boss-node__mastery">{masteryPct}%</span>
        )}
      </div>
    )
  }
  ```

  ```typescript
  // components/progression/ProgressionMap.tsx
  import { BossNode } from './BossNode'

  interface Boss { id: string; name: string; status: 'defeated' | 'current' | 'locked'; masteryPct: number }

  /** Visual trail of boss progression — defeated → current → locked. */
  export function ProgressionMap({ bosses, unlockThreshold = 60 }: { bosses: Boss[]; unlockThreshold?: number }) {
    return (
      <div className="progression-map">
        {bosses.map((boss, i) => (
          <div key={boss.id} className="progression-map__stop">
            <BossNode {...boss} unlockThreshold={unlockThreshold} />
            {i < bosses.length - 1 && <div className="progression-map__connector" />}
          </div>
        ))}
      </div>
    )
  }
  ```

- [ ] **Step 3: Run tests**

  ```bash
  npm test -- __tests__/progression/ProgressionMap.test.tsx
  ```

- [ ] **Step 4: Commit**

  ```bash
  git commit -am "feat(tl-7wv): boss progression map — visual trail, defeated/current/locked states, unlock threshold display

  Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
  git push
  ```

---

## TRACK C: Phase 8 Hardening

### Task C1: JWT Audience Validation (hq-cv-gjapg / alai)

**Bead:** `hq-cv-gjapg`  
**Owner:** alai (after PR #145 merges)  
**Branch:** `feat/hq-cv-gjapg-jwt-aud`

**Files:**
- Modify: `services/tutoring-service/app/auth.py` — enable `verify_aud=True`
- Modify: `services/user-service/internal/auth/jwt.go` — set `aud` claim on token issuance
- Test: `services/tutoring-service/tests/test_auth.py` — verify aud validation rejects wrong audience
- Test: `services/user-service/internal/auth/jwt_test.go` — verify aud set correctly

- [ ] **Step 1: Write failing test for tutoring-service aud validation**

  ```python
  # tests/test_auth.py (add to existing)
  def test_rejects_token_with_wrong_audience(client):
      """Token issued for a different service must be rejected."""
      token = create_test_token(aud="other-service")
      resp = client.get("/v1/sessions", headers={"Authorization": f"Bearer {token}"})
      assert resp.status_code == 401
      assert "audience" in resp.json()["detail"].lower()

  def test_accepts_token_with_correct_audience(client, auth_headers):
      """Token with correct aud claim must pass."""
      resp = client.get("/v1/sessions", headers=auth_headers)
      assert resp.status_code == 200
  ```

- [ ] **Step 2: Enable verify_aud in tutoring-service auth.py**

  Find the JWT decode call in `app/auth.py` and set:
  ```python
  payload = jwt.decode(
      token,
      settings.jwt_secret,
      algorithms=["HS256"],
      options={"verify_aud": True},
      audience="tutoring-service",   # must match aud claim set by user-service
  )
  ```

- [ ] **Step 3: Write failing test for user-service aud claim**

  ```go
  // internal/auth/jwt_test.go (add)
  func TestGenerateToken_SetsAudClaim(t *testing.T) {
      token, err := GenerateToken(userID, email, "standard", "active")
      require.NoError(t, err)
      claims, err := ParseToken(token)
      require.NoError(t, err)
      assert.Contains(t, claims.Audience, "tutoring-service")
  }
  ```

- [ ] **Step 4: Set aud claim in user-service jwt.go**

  In `GenerateToken`, add to claims:
  ```go
  RegisteredClaims: jwt.RegisteredClaims{
      Audience: jwt.ClaimStrings{"tutoring-service"},
      // ... existing claims
  }
  ```

- [ ] **Step 5: Run both service tests**

  ```bash
  cd services/tutoring-service && pytest tests/test_auth.py -v
  cd services/user-service && go test ./internal/auth/... -v
  ```

- [ ] **Step 6: Commit**

  ```bash
  git commit -am "feat(hq-cv-gjapg): enable JWT audience validation in tutoring-service + set aud in user-service

  Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
  git push
  ```

---

## Merge Order (Critical Path)

```
PR #213 (context window)   ──► merge now (Stilgar approval needed)
PR #211 (boss backend CI)  ──► merge after lint fix (carn)
PR #145 (upload UI)        ──► merge after Codecov fix (alai)
    │
    ├─► hq-cv-iguk6 (dink) — PDF pipeline
    ├─► hq-cv-kqszc (bean) — Qdrant Helm
    └─► hq-cv-gjapg (alai) — JWT aud
        │
        └─► tl-2l7 (shen)  — battle state machine  [after #211]
            └─► tl-dye (shen) — Three.js + WebSocket
                └─► tl-7wv (shen) — progression map
```

---

## Definition of Done

- [ ] Phase 2 exit criteria: upload chemistry PDF → "what is a chiral center?" → answer with page reference
- [ ] Phase 4 exit criteria: complete chapter → fight animated boss → earn loot on victory
- [ ] Phase 8 hardening: JWT aud validation active, OTel upgraded, Codecov ≥80% on all patches
- [ ] All PRs: TDD compliant, docstrings present, Codecov patch ≥80%, lint clean

---

*Plan authored by Petra (teachers_lounge PM) · 2026-04-12*  
*Spec reference: docs/tv-tutor-design.md § 14 Phased Build Plan*
