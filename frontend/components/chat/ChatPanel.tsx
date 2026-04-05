'use client'

import { useState, useRef, useEffect, useCallback } from 'react'
import ChatMessage, { type Message } from './ChatMessage'
import ChatInput from './ChatInput'

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

export default function ChatPanel() {
  const [messages, setMessages] = useState<Message[]>([WELCOME_MESSAGE])
  const [input, setInput] = useState('')
  const [isStreaming, setIsStreaming] = useState(false)
  const bottomRef = useRef<HTMLDivElement>(null)
  const abortRef = useRef<AbortController | null>(null)

  // Auto-scroll on new content
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const sendMessage = useCallback(async () => {
    const content = input.trim()
    if (!content || isStreaming) return

    setInput('')

    const userMsg: Message = { id: newId(), role: 'user', content }
    const assistantId = newId()
    const assistantMsg: Message = {
      id: assistantId,
      role: 'assistant',
      content: '',
      streaming: true,
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
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        const current = buffer
        setMessages((prev) =>
          prev.map((m) => (m.id === assistantId ? { ...m, content: current } : m)),
        )
      }

      // Mark streaming done
      setMessages((prev) =>
        prev.map((m) => (m.id === assistantId ? { ...m, streaming: false } : m)),
      )
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
  }, [input, messages, isStreaming])

  return (
    <div className="flex flex-col h-full bg-bg-deep">
      {/* Chat header */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-border-dim bg-bg-panel flex-shrink-0">
        <div className="flex items-center gap-2">
          <span className="text-xs font-medium text-text-dim">Session</span>
          <span className="w-1 h-1 rounded-full bg-border-mid" />
          <span className="font-mono text-xs text-text-dim">Organic Chemistry 101</span>
        </div>
        <div className="flex items-center gap-1.5">
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
        <div ref={bottomRef} />
      </div>

      {/* Input */}
      <ChatInput value={input} onChange={setInput} onSubmit={sendMessage} disabled={isStreaming} />
    </div>
  )
}
