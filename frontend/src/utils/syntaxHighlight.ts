import { common, createLowlight } from 'lowlight'

type HighlightNode = {
  type: string
  value?: string
  tagName?: string
  properties?: Record<string, unknown>
  children?: HighlightNode[]
}

export const lowlight = createLowlight(common)

function renderNode(node: HighlightNode): Node {
  if (node.type === 'text') return document.createTextNode(node.value ?? '')

  const element = document.createElement('span')
  const classNames = node.properties?.className
  const safeClassNames = (Array.isArray(classNames) ? classNames : [classNames])
    .filter((value): value is string => typeof value === 'string' && /^hljs-[\w-]+$/.test(value))

  if (safeClassNames.length) element.classList.add(...safeClassNames)
  element.append(...(node.children ?? []).map(renderNode))
  return element
}

export function highlightCodeBlocks(root: ParentNode) {
  root.querySelectorAll<HTMLElement>('pre code').forEach((code) => {
    if (code.dataset.highlighted === 'true') return

    const language = [...code.classList]
      .find((className) => className.startsWith('language-'))
      ?.slice('language-'.length)
    const result = language && lowlight.registered(language)
      ? lowlight.highlight(language, code.textContent ?? '')
      : lowlight.highlightAuto(code.textContent ?? '')

    code.replaceChildren(...(result.children as HighlightNode[]).map(renderNode))
    code.dataset.highlighted = 'true'
  })
}
