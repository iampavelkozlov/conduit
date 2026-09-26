// @vitest-environment jsdom

import { beforeEach, describe, expect, it, vi } from 'vitest'

import { initializeTheme, useTheme } from './useTheme'

describe('theme', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.className = ''
    document.documentElement.style.colorScheme = ''
    vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: false })))
  })

  it('restores a saved dark theme', () => {
    localStorage.setItem('conduit_theme', 'dark')

    initializeTheme()

    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(document.documentElement.style.colorScheme).toBe('dark')
  })

  it('uses the operating system preference on first visit', () => {
    vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: true })))

    initializeTheme()

    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(localStorage.getItem('conduit_theme')).toBe('dark')
  })

  it('toggles and persists the selected theme', () => {
    localStorage.setItem('conduit_theme', 'light')
    initializeTheme()

    useTheme().toggleTheme()

    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(localStorage.getItem('conduit_theme')).toBe('dark')
  })
})
