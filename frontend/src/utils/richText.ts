import DOMPurify from 'dompurify'

const supportedHtml = /<\/?(?:a|blockquote|br|code|em|h[1-6]|hr|li|ol|p|pre|s|strong|u|ul)\b/i

function escapeHtml(value: string) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#039;')
}

export function normalizeRichText(value: string | null | undefined) {
  const content = value?.trim() ?? ''
  if (!content) return ''
  if (supportedHtml.test(content)) return content

  return content
    .split(/\n{2,}/)
    .map((paragraph) => `<p>${escapeHtml(paragraph).replaceAll('\n', '<br>')}</p>`)
    .join('')
}

export function sanitizeRichText(value: string | null | undefined) {
  return String(DOMPurify.sanitize(normalizeRichText(value), {
    USE_PROFILES: { html: true },
    ALLOWED_ATTR: ['href', 'rel', 'target'],
  }))
}

export function isRichTextEmpty(value: string | null | undefined) {
  const document = new DOMParser().parseFromString(sanitizeRichText(value), 'text/html')
  return !document.body.textContent?.trim()
}
