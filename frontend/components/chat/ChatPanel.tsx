'use client'

import { useState, useRef, useEffect, useCallback } from 'react'
import ChatMessage, { type DiagramAttachment, type Message } from './ChatMessage'
import ChatInput from './ChatInput'
import MoleculeBuilder from './MoleculeBuilder'
import LearningStyleBadge from './LearningStyleBadge'

const WELCOME_MESSAGE: Message = {
  id: 'welcome',
  role: 'assistant',
  content: `Welcome back! I'm **Professor Nova**, your personal AI tutor. 🎓

I'm here to help you master your course material. Upload a document in the sidebar, then ask me anything — I'll provide personalized explanations based on your learning style and history.

*What would you like to explore today?*`,
}

let msgCounter = 0
function newId() {
  return `msg-${++msgCounter}-${Date.now()}`
}

// SSE event shapes emitted by the tutoring service
interface SseEvent {
  type: 'delta' | 'sources' | 'done' | 'error' | 'diagram' | 'molecule_builder'
  content?: string
  message_id?: string
  sources?: unknown[]
  diagram?: DiagramAttachment
}

// Felder-Silverman dials from the user service
interface FelderDials {
  active_reflective: number
  sensing_intuitive: number
  visual_verbal: number
  sequential_global: number
}

/** active_reflective < -0.2 → kinesthetic/active learner → show molecule builder */
function isKinesthetic(dials: FelderDials | null): boolean {
  return (dials?.active_reflective ?? 0) < -0.2
}

export default function ChatPanel() {
  const [messages, setMessages] = useState<Message[]>([WELCOME_MESSAGE])
  const [input, setInput] = useState('')
  const [isStreaming, setIsStreaming] = useState(false)
  const [dials, setDials] = useState<FelderDials | null>(null)
  const [showMoleculeBuilder, setShowMoleculeBuilder] = useState(false)
  const [moleculeHint, setMoleculeHint] = useState<string | undefined>(undefined)
  const bottomRef = useRef<HTMLDivElement>(null)
  const abortRef = useRef<AbortController | null>(null)

  // Auto-scroll on new content
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  /**
   * Parse SSE lines from the streaming response.
   *
   * Each line has the form ``data: <json>``.  Accumulates delta tokens into
   * the message content and appends diagrams when ``diagram`` events arrive.
   * Shows the molecule builder when ``active_reflective < -0.2``.
   */
  const sendMessage = useCallback(async () => {
    const content = input.trim()
    if (!content || isStreaming) return

    setInput('')
    setShowMoleculeBuilder(false)

    const userMsg: Message = { id: newId(), role: 'user', content }
    const assistantId = newId()
    const assistantMsg: Message = {
      id: assistantId,
      role: 'assistant',
      content: '',
      streaming: true,
      diagrams: [],
    }

    setMessages((prev) => [...prev, userMsg, assistantMsg])
    setIsStreaming(true)

    const controller = new AbortController()
    abortRef.current = controller

    try {
      const res = await fetch('/api/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          messages: [...messages, userMsg].map((m) => ({
            role: m.role,
            content: m.content,
          })),
        }),
        signal: controller.signal,
      })

      if (!res.ok || !res.body) {
        throw new Error(`API error: ${res.status}`)
      }

      const reader = res.body.getReader()
      const decoder = new TextDecoder()
      let rawBuffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        rawBuffer += decoder.decode(value, { stream: true })
        const lines = rawBuffer.split('\n')
        rawBuffer = lines.pop() ?? '' // last (possibly incomplete) line stays in buffer

        for (const line of lines) {
          if (!line.startsWith('data: ')) continue
          const json = line.slice(6).trim()
          if (!json) continue

          let event: SseEvent
          try {
            event = JSON.parse(json) as SseEvent
          } catch {
            continue
          }

          if (event.type === 'delta') {
            setMessages((prev) =>
              prev.map((m) =>
                m.id === assistantId ? { ...m, content: m.content + (event.content ?? '') } : m,
              ),
            )
          } else if (event.type === 'diagram' && event.diagram) {
            const diagram = event.diagram
            setMessages((prev) =>
              prev.map((m) =>
                m.id === assistantId ? { ...m, diagrams: [...(m.diagrams ?? []), diagram] } : m,
              ),
            )
          }
        }
      }

      // Mark streaming done
      setMessages((prev) =>
        prev.map((m) => (m.id === assistantId ? { ...m, streaming: false } : m)),
      )

      // Show molecule builder for kinesthetic learners asking structural questions
      if (isKinesthetic(dials) && _isStructuralQuestion(content)) {
        setMoleculeHint(`Draw the structure described in your question.`)
        setShowMoleculeBuilder(true)
      }
    } catch (err: unknown) {
      if (err instanceof Error && err.name === 'AbortError') return
      setMessages((prev) =>
        prev.map((m) =>
          m.id === assistantId
            ? { ...m, content: 'Sorry, something went wrong. Please try again.', streaming: false }
            : m,
        ),
      )
    } finally {
      setIsStreaming(false)
      abortRef.current = null
    }
  }, [input, messages, isStreaming, dials])

  const handleMoleculeSubmit = useCallback(async (smiles: string) => {
    setShowMoleculeBuilder(false)
    // Post the SMILES answer to the quiz endpoint and add a user message showing it
    const userMsg: Message = {
      id: newId(),
      role: 'user',
      content: `[Molecule answer] \`${smiles}\``,
    }
    setMessages((prev) => [...prev, userMsg])

    try {
      await fetch('/api/quiz/answer', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ smiles_answer: smiles }),
      })
    } catch {
      // Non-fatal — the tutor will respond in the next turn
    }
  }, [])

  return (
    <div className="flex flex-col h-full bg-bg-deep">
      {/* Chat header */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-border-dim bg-bg-panel flex-shrink-0">
        <div className="flex items-center gap-2">
          <span className="text-xs font-medium text-text-dim">Session</span>
          <span className="w-1 h-1 rounded-full bg-border-mid" />
          <span className="font-mono text-xs text-text-dim">Organic Chemistry 101</span>
        </div>
        <div className="flex items-center gap-2">
          <LearningStyleBadge dials={dials} />
          <span className="w-1.5 h-1.5 rounded-full bg-neon-green animate-pulse-slow" />
          <span className="text-[10px] text-text-dim">Connected</span>
        </div>
      </div>

      {/* Message list */}
      <div className="flex-1 overflow-y-auto px-4 py-4 flex flex-col gap-4 min-h-0">
        {messages.map((msg) => (
          <ChatMessage key={msg.id} message={msg} />
        ))}
        {isStreaming && (
          <div className="flex gap-2 items-center text-xs text-text-dim animate-fade-in">
            <span className="text-sm">🤖</span>
            <div className="flex gap-1">
              <span
                className="w-1.5 h-1.5 rounded-full bg-neon-blue animate-bounce"
                style={{ animationDelay: '0ms' }}
              />
              <span
                className="w-1.5 h-1.5 rounded-full bg-neon-blue animate-bounce"
                style={{ animationDelay: '150ms' }}
              />
              <span
                className="w-1.5 h-1.5 rounded-full bg-neon-blue animate-bounce"
                style={{ animationDelay: '300ms' }}
              />
            </div>
          </div>
        )}

        {/* Molecule builder — shown after a structural question for kinesthetic learners */}
        {showMoleculeBuilder && (
          <div className="animate-slide-up">
            <MoleculeBuilder onSubmit={handleMoleculeSubmit} hint={moleculeHint} />
          </div>
        )}

        <div ref={bottomRef} />
      </div>

      {/* Input */}
      <ChatInput value={input} onChange={setInput} onSubmit={sendMessage} disabled={isStreaming} />
    </div>
  )
}

/** True when the user's question is likely about molecular/chemical structure. */
function _isStructuralQuestion(text: string): boolean {
  return /\b(structure|molecule|draw|benzene|ring|bond|formula|compound|organic|atom)\b/i.test(text)
}
