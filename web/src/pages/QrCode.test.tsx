import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { QrCode } from './QrCode.tsx'

describe('QrCode', () => {
  it('renders an svg for the url', () => {
    render(<QrCode url="https://mol.la/abc1234" code="abc1234" />)
    expect(screen.getByTitle('QR code for https://mol.la/abc1234').closest('svg')).toBeInTheDocument()
  })

  it('downloads the canvas as a png named after the code', () => {
    const blob = new Blob(['png'], { type: 'image/png' })
    vi.spyOn(HTMLCanvasElement.prototype, 'toBlob').mockImplementation((cb) => cb(blob))
    const createObjectURL = vi.fn().mockReturnValue('blob:molla')
    const revokeObjectURL = vi.fn()
    vi.stubGlobal('URL', Object.assign(URL, { createObjectURL, revokeObjectURL }))
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined)

    render(<QrCode url="https://mol.la/abc1234" code="abc1234" />)
    fireEvent.click(screen.getByRole('button', { name: 'Download PNG' }))

    expect(createObjectURL).toHaveBeenCalledWith(blob)
    expect(click).toHaveBeenCalledOnce()
    const anchor = click.mock.instances[0] as HTMLAnchorElement
    expect(anchor.download).toBe('molla-abc1234.png')
    expect(anchor.href).toBe('blob:molla')
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:molla')
  })
})
