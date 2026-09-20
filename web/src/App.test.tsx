import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { App } from './App.tsx'

describe('App', () => {
  it('shows the create page at /app/', () => {
    window.history.replaceState(null, '', '/app/')
    render(<App />)
    expect(screen.getByRole('heading', { name: 'Shorten a long link' })).toBeInTheDocument()
    expect(screen.getByText('dev')).toBeInTheDocument()
  })

  it('shows about, developers, terms, and privacy', () => {
    window.history.replaceState(null, '', '/app/about')
    const about = render(<App />)
    expect(screen.getByRole('heading', { name: 'About' })).toBeInTheDocument()
    about.unmount()

    window.history.replaceState(null, '', '/app/developers')
    const developers = render(<App />)
    expect(screen.getByRole('heading', { name: 'Developers' })).toBeInTheDocument()
    developers.unmount()

    window.history.replaceState(null, '', '/app/terms')
    const terms = render(<App />)
    expect(screen.getByRole('heading', { name: 'Terms and conditions' })).toBeInTheDocument()
    terms.unmount()

    window.history.replaceState(null, '', '/app/privacy')
    render(<App />)
    expect(screen.getByRole('heading', { name: 'Privacy' })).toBeInTheDocument()
  })
})
