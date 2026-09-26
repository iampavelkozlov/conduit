// @vitest-environment jsdom

import { describe, expect, it } from 'vitest'

import { highlightCodeBlocks } from './syntaxHighlight'

describe('syntax highlighting', () => {
  it('highlights a code block with its declared language', () => {
    const root = document.createElement('div')
    root.innerHTML = '<pre><code class="language-js">const answer = 42</code></pre>'

    highlightCodeBlocks(root)

    const code = root.querySelector('code')
    expect(code?.dataset.highlighted).toBe('true')
    expect(code?.querySelector('.hljs-keyword')?.textContent).toBe('const')
    expect(code?.textContent).toBe('const answer = 42')
  })

  it('automatically detects syntax when a language is not declared', () => {
    const root = document.createElement('div')
    root.innerHTML = '<pre><code>function greet() { return "hello" }</code></pre>'

    highlightCodeBlocks(root)

    expect(root.querySelector('code')?.querySelector('[class^="hljs-"]')).not.toBeNull()
  })
})
