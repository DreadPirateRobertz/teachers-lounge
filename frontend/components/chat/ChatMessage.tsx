export interface Message {
  id: string
  role: 'user' | 'assistant'
  content: string
  streaming?: boolean
}

interface Props {
  message: Message
}

// Very lightweight markdown: bold, italic, code blocks, inline code, line breaks
function renderContent(text: string) {
  const lines = text.split('\n')
  return lines.map((line, i) => (
    <span key={i}>
      {renderInline(line)}
      {i < lines.length - 1 && <br />}
    </span>
  ))
}

function renderInline(text: string) {
  // Split on **bold**, *italic*, `code`
  const parts = text.split(/(\*\*[^*]+\*\*|\*[^*]+\*|`[^`]+`)/g)
  return parts.map((part, i) => {
    if (part.startsWith('**') && part.endsWith('**')) {
      return <strong key={i} className="font-bold text-text-bright">{part.slice(2, -2)}</strong>
    }
    if (part.startsWith('*') && part.endsWith('*')) {
      return <em key={i} className="italic text-text-base">{part.slice(1, -1)}</em>
    }
    if (part.startsWith('`') && part.endsWith('`')) {
      return (
        <code key={i} className="font-mono text-xs bg-bg-deep border border-border-dim px-1 py-0.5 rounded text-neon-green">
          {part.slice(1, -1)}
        </code>
      )
    }
    return <span key={i}>{part}</span>
  })
}

export default function ChatMessage({ message }: Props) {
  const isUser = message.role === 'user'

  return (
    <div className={`flex gap-3 animate-slide-up ${isUser ? 'flex-row-reverse' : 'flex-row'}`}>
      {/* Avatar */}
      <div className={`flex-shrink-0 w-7 h-7 rounded-full flex items-center justify-center text-sm border ${
        isUser
          ? 'bg-bg-card border-border-mid'
          : 'bg-neon-blue/10 border-neon-blue/30 shadow-neon-blue-sm'
      }`}>
        {isUser ? '🧙' : '🤖'}
      </div>

      {/* Bubble */}
      <div className={`max-w-[75%] rounded-xl px-3.5 py-2.5 text-sm leading-relaxed ${
        isUser
          ? 'bg-bg-card border border-border-mid text-text-base rounded-tr-sm'
          : 'bg-neon-blue/5 border border-neon-blue/20 text-text-base rounded-tl-sm'
      }`}>
        {!isUser && (
          <div className="text-[10px] font-mono text-neon-blue mb-1.5 font-bold">
            PROF NOVA
          </div>
        )}
        <div className={message.streaming ? 'typing-cursor' : ''}>
          {renderContent(message.content)}
        </div>
      </div>
    </div>
  )
}
