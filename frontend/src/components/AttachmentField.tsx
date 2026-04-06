import { ChangeEvent, DragEvent, useEffect, useRef, useState } from 'react'
import { readApiError, uploadAttachments } from '../services/api'
import type { UploadSourceFile } from '../services/api'
import type { UploadAttachment } from '../types'

interface AttachmentFieldProps {
  inputId?: string
  value: UploadAttachment[]
  onChange: (value: UploadAttachment[]) => void
  disabled?: boolean
}

const formatFileSize = (size: number) => {
  if (!size) return '0 B'
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(2)} KB`
  if (size < 1024 * 1024 * 1024) return `${(size / (1024 * 1024)).toFixed(2)} MB`
  return `${(size / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

const mergeAttachments = (origin: UploadAttachment[], incoming: UploadAttachment[]) => {
  const map = new Map<string, UploadAttachment>()
  origin.forEach((item) => {
    if (item.filePath) map.set(item.filePath, item)
  })
  incoming.forEach((item) => {
    if (item.filePath) map.set(item.filePath, item)
  })
  return Array.from(map.values())
}

const toSourceFiles = (files: FileList) => {
  return Array.from(files).map((file) => {
    const relativePath = (file as File & { webkitRelativePath?: string }).webkitRelativePath || file.name
    return { file, relativePath } as UploadSourceFile
  })
}

const readAllDirectoryEntries = async (reader: { readEntries: (callback: (entries: any[]) => void, errorCallback?: (error: unknown) => void) => void }) => {
  const allEntries: any[] = []
  for (;;) {
    const chunk = await new Promise<any[]>((resolve, reject) => {
      reader.readEntries((entries) => resolve(entries || []), reject)
    })
    if (chunk.length === 0) break
    allEntries.push(...chunk)
  }
  return allEntries
}

const collectEntryFiles = async (entry: any, currentPath: string, result: UploadSourceFile[]) => {
  if (!entry) return
  if (entry.isFile) {
    const file = await new Promise<File>((resolve, reject) => {
      entry.file((selected: File) => resolve(selected), reject)
    })
    const relativePath = currentPath ? `${currentPath}/${file.name}` : ((file as File & { webkitRelativePath?: string }).webkitRelativePath || file.name)
    result.push({ file, relativePath })
    return
  }
  if (!entry.isDirectory) return
  const nextPath = currentPath ? `${currentPath}/${entry.name}` : entry.name
  const children = await readAllDirectoryEntries(entry.createReader())
  for (const child of children) {
    await collectEntryFiles(child, nextPath, result)
  }
}

const collectDroppedFiles = async (event: DragEvent<HTMLDivElement>) => {
  const items = Array.from(event.dataTransfer.items || [])
  if (items.length === 0) {
    return toSourceFiles(event.dataTransfer.files)
  }

  const entries = items
    .map((item) => ((item as DataTransferItem & { webkitGetAsEntry?: () => any }).webkitGetAsEntry?.()))
    .filter(Boolean)

  if (entries.length === 0) {
    return toSourceFiles(event.dataTransfer.files)
  }

  const files: UploadSourceFile[] = []
  for (const entry of entries) {
    await collectEntryFiles(entry, '', files)
  }
  if (files.length > 0) {
    return files
  }
  return toSourceFiles(event.dataTransfer.files)
}

export function AttachmentField({ inputId, value, onChange, disabled = false }: AttachmentFieldProps) {
  const [uploading, setUploading] = useState(false)
  const [dragging, setDragging] = useState(false)
  const [error, setError] = useState('')
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const folderInputRef = useRef<HTMLInputElement | null>(null)

  useEffect(() => {
    if (!folderInputRef.current) return
    folderInputRef.current.setAttribute('webkitdirectory', '')
    folderInputRef.current.setAttribute('directory', '')
  }, [])

  const uploadSourceFiles = async (files: UploadSourceFile[]) => {
    if (disabled) return
    if (files.length === 0) return
    try {
      setUploading(true)
      setError('')
      const attachments = await uploadAttachments(files)
      onChange(mergeAttachments(value, attachments))
    } catch (uploadError) {
      setError(readApiError(uploadError, '上传文件失败'))
    } finally {
      setUploading(false)
    }
  }

  const handleFileInputChange = async (event: ChangeEvent<HTMLInputElement>) => {
    const files = event.target.files
    if (!files || files.length === 0) return
    await uploadSourceFiles(toSourceFiles(files))
    event.target.value = ''
  }

  const removeAttachment = (target: UploadAttachment) => {
    if (disabled) return
    onChange(value.filter((item) => item.filePath !== target.filePath))
  }

  return (
    <div className="attachment-field">
      <input id={inputId} ref={fileInputRef} className="sr-only" type="file" multiple onChange={(event) => { void handleFileInputChange(event) }} disabled={disabled || uploading} />
      <input ref={folderInputRef} className="sr-only" type="file" multiple onChange={(event) => { void handleFileInputChange(event) }} disabled={disabled || uploading} />
      <div className="attachment-toolbar">
        <button type="button" className="btn secondary" onClick={() => fileInputRef.current?.click()} disabled={disabled || uploading}>上传多个文件</button>
        <button type="button" className="btn secondary" onClick={() => folderInputRef.current?.click()} disabled={disabled || uploading}>上传文件夹（自动压缩 ZIP）</button>
      </div>
      <div
        className={`attachment-dropzone${dragging ? ' active' : ''}`}
        onDragEnter={(event) => { event.preventDefault(); if (!disabled) setDragging(true) }}
        onDragOver={(event) => { event.preventDefault(); if (!disabled) setDragging(true) }}
        onDragLeave={(event) => { event.preventDefault(); setDragging(false) }}
        onDrop={(event) => {
          event.preventDefault()
          setDragging(false)
          if (disabled) return
          void collectDroppedFiles(event).then((files) => uploadSourceFiles(files)).catch(() => setError('读取拖放文件失败'))
        }}
      >
        {disabled ? '当前账号无上传权限' : (uploading ? '正在上传，请稍候...' : '将文件或文件夹拖放到这里上传（文件夹会自动压缩为 ZIP）')}
      </div>
      {value.length > 0 && (
        <div className="attachment-list">
          {value.map((item) => (
            <div key={item.filePath} className="attachment-item">
              <a href={item.filePath} target="_blank" rel="noreferrer">{item.relativePath || item.fileName || '附件'}</a>
              <span>{formatFileSize(item.fileSize)}</span>
              <button type="button" className="btn secondary" onClick={() => removeAttachment(item)} disabled={disabled}>移除</button>
            </div>
          ))}
        </div>
      )}
      {error && <p className="error">{error}</p>}
    </div>
  )
}
