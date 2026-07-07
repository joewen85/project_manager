import { useEffect } from 'react'
import { X } from 'lucide-react'
import type { UploadAttachment } from '../types'

interface ImagePreviewOverlayProps {
  image: UploadAttachment | null
  onClose: () => void
}

export function ImagePreviewOverlay({ image, onClose }: ImagePreviewOverlayProps) {
  useEffect(() => {
    if (!image) return
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [image, onClose])

  if (!image) return null

  const label = image.relativePath || image.fileName || '登记项图片'

  return (
    <div className="image-preview-overlay" role="dialog" aria-modal="true" aria-label={label} onClick={onClose}>
      <div className="image-preview-panel" onClick={(event) => event.stopPropagation()}>
        <button type="button" className="image-preview-close" onClick={onClose} autoFocus aria-label="关闭图片预览">
          <X size={18} aria-hidden="true" />
        </button>
        <img src={image.filePath} alt={label} />
        <div className="image-preview-caption">{label}</div>
      </div>
    </div>
  )
}
