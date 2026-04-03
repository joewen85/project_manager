import { ReactNode } from 'react'

interface ModalProps {
  open: boolean
  title: string
  onClose: () => void
  children: ReactNode
}

export function Modal({ open, title, onClose, children }: ModalProps) {
  if (!open) return null

  return (
    <div className="modal-mask" role="dialog" aria-modal="true" aria-label={title}>
      <div className="modal-card card">
        <div className="modal-header">
          <h3>{title}</h3>
          <button className="btn secondary" onClick={onClose}>关闭</button>
        </div>
        <div className="modal-body">
          {children}
        </div>
      </div>
    </div>
  )
}
