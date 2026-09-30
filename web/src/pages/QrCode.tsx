import { useRef } from 'react'
import { QRCodeCanvas, QRCodeSVG } from 'qrcode.react'

const DOWNLOAD_SIZE = 1024

type QrCodeProps = {
  url: string
  code: string
}

export function QrCode({ url, code }: QrCodeProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)

  function download() {
    canvasRef.current?.toBlob((blob) => {
      if (!blob) {
        return
      }
      const href = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = href
      anchor.download = `molla-${code}.png`
      anchor.click()
      URL.revokeObjectURL(href)
    }, 'image/png')
  }

  return (
    <div>
      <p className="text-xs font-medium tracking-[0.04em] text-muted-foreground uppercase">QR code</p>
      <QRCodeSVG
        value={url}
        title={`QR code for ${url}`}
        size={160}
        marginSize={2}
        bgColor="#ffffff"
        fgColor="#000000"
        className="mt-3 border border-border"
      />
      <QRCodeCanvas
        ref={canvasRef}
        value={url}
        size={DOWNLOAD_SIZE}
        marginSize={2}
        bgColor="#ffffff"
        fgColor="#000000"
        className="hidden"
        aria-hidden="true"
      />
      <button
        type="button"
        className="mt-3 inline-flex h-9 items-center border border-input px-3.5 text-sm hover:border-brand hover:glow"
        onClick={download}
      >
        Download PNG
      </button>
    </div>
  )
}
