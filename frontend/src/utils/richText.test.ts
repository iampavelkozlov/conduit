// @vitest-environment jsdom

import { describe, expect, it } from 'vitest'

import { isRichTextEmpty, normalizeRichText, sanitizeRichText } from './richText'

describe('rich text utilities', () => {
  it('converts legacy plain text into paragraphs and preserves line breaks', () => {
    expect(normalizeRichText('First line\nSecond line\n\nNext paragraph')).toBe(
      '<p>First line<br>Second line</p><p>Next paragraph</p>',
    )
  })

  it('escapes markup in legacy plain text', () => {
    expect(normalizeRichText('2 < 3 & 4 > 1')).toBe('<p>2 &lt; 3 &amp; 4 &gt; 1</p>')
  })

  it('removes scripts, event handlers, and unsafe links', () => {
    const result = sanitizeRichText(
      '<p onclick="alert(1)">Safe</p><script>alert(1)</script><a href="javascript:alert(1)">Link</a>',
    )

    expect(result).toBe('<p>Safe</p><a>Link</a>')
  })

  it('preserves supported rich formatting', () => {
    expect(sanitizeRichText('<h2>Heading</h2><p><strong>Bold</strong> and <em>italic</em></p>')).toBe(
      '<h2>Heading</h2><p><strong>Bold</strong> and <em>italic</em></p>',
    )
  })

  it('preserves only a safe language class on code blocks', () => {
    expect(sanitizeRichText(
      '<p class="hidden">Text</p><pre><code class="language-js injected">const value = 1</code></pre>',
    )).toBe('<p>Text</p><pre><code class="language-js">const value = 1</code></pre>')
  })

  it.each(['', '<p></p>', '<p>   </p>'])('recognizes empty content: %s', (content) => {
    expect(isRichTextEmpty(content)).toBe(true)
  })

  it('recognizes formatted text as non-empty', () => {
    expect(isRichTextEmpty('<p><strong>Content</strong></p>')).toBe(false)
  })
})
