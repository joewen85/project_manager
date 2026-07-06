import { Fragment, ReactNode } from 'react'

// Renders a constrained Markdown subset (headings, bullet/ordered lists,
// bold/inline-code, paragraphs) to safe React nodes. We deliberately avoid
// dangerouslySetInnerHTML so AI-generated drafts can never inject markup.

function renderInline(text: string, keyPrefix: string): ReactNode[] {
  const nodes: ReactNode[] = []
  // Split on **bold** and `code` while keeping delimiters.
  const pattern = /(\*\*[^*]+\*\*|`[^`]+`)/g
  const parts = text.split(pattern).filter((part) => part !== '')
  parts.forEach((part, index) => {
    const key = `${keyPrefix}-${index}`
    if (part.startsWith('**') && part.endsWith('**')) {
      nodes.push(<strong key={key}>{part.slice(2, -2)}</strong>)
    } else if (part.startsWith('`') && part.endsWith('`')) {
      nodes.push(<code key={key}>{part.slice(1, -1)}</code>)
    } else {
      nodes.push(<Fragment key={key}>{part}</Fragment>)
    }
  })
  return nodes
}

export function MarkdownView({ content, className }: { content: string; className?: string }) {
  const lines = content.replace(/\r\n/g, '\n').split('\n')
  const blocks: ReactNode[] = []
  let listItems: string[] = []
  let listType: 'ul' | 'ol' | null = null
  let paragraph: string[] = []

  const flushList = () => {
    if (listItems.length === 0 || !listType) return
    const items = listItems
    const key = `list-${blocks.length}`
    const children = items.map((item, index) => (
      <li key={`${key}-${index}`}>{renderInline(item, `${key}-${index}`)}</li>
    ))
    blocks.push(listType === 'ol' ? <ol key={key}>{children}</ol> : <ul key={key}>{children}</ul>)
    listItems = []
    listType = null
  }

  const flushParagraph = () => {
    if (paragraph.length === 0) return
    const key = `p-${blocks.length}`
    blocks.push(<p key={key}>{renderInline(paragraph.join(' '), key)}</p>)
    paragraph = []
  }

  for (const rawLine of lines) {
    const line = rawLine.trimEnd()
    const trimmed = line.trim()

    if (trimmed === '') {
      flushList()
      flushParagraph()
      continue
    }

    const heading = /^(#{1,4})\s+(.*)$/.exec(trimmed)
    if (heading) {
      flushList()
      flushParagraph()
      const level = heading[1].length
      const text = heading[2]
      const key = `h-${blocks.length}`
      const inner = renderInline(text, key)
      if (level <= 1) blocks.push(<h4 key={key} className="markdown-h1">{inner}</h4>)
      else if (level === 2) blocks.push(<h5 key={key} className="markdown-h2">{inner}</h5>)
      else blocks.push(<h6 key={key} className="markdown-h3">{inner}</h6>)
      continue
    }

    const bullet = /^[-*]\s+(.*)$/.exec(trimmed)
    if (bullet) {
      flushParagraph()
      if (listType !== 'ul') flushList()
      listType = 'ul'
      listItems.push(bullet[1])
      continue
    }

    const ordered = /^\d+\.\s+(.*)$/.exec(trimmed)
    if (ordered) {
      flushParagraph()
      if (listType !== 'ol') flushList()
      listType = 'ol'
      listItems.push(ordered[1])
      continue
    }

    flushList()
    paragraph.push(trimmed)
  }
  flushList()
  flushParagraph()

  return <div className={className ? `markdown-view ${className}` : 'markdown-view'}>{blocks}</div>
}
