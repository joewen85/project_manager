import { ChangeEvent, DragEvent, useRef, useState } from 'react'
import { ImagePlus, Sparkles, Trash2, UploadCloud } from 'lucide-react'
import { ImagePreviewOverlay } from './ImagePreviewOverlay'
import { api, readApiError, uploadAttachments } from '../services/api'
import type { UploadSourceFile } from '../services/api'
import type { ProjectRegisterImage, UploadAttachment } from '../types'

interface ImageAttachmentFieldProps {
  inputId?: string
  value: ProjectRegisterImage[]
  onChange: (value: ProjectRegisterImage[]) => void
  disabled?: boolean
  uploadDisabled?: boolean
  projectId?: string | number
  registerId?: number
  canGenerateDescription?: boolean
}

const maxImageSize = 50 * 1024 * 1024

const formatFileSize = (size: number) => {
  if (!size) return '0 B'
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(2)} KB`
  return `${(size / (1024 * 1024)).toFixed(2)} MB`
}

const mergeImages = (origin: ProjectRegisterImage[], incoming: UploadAttachment[]) => {
  const map = new Map<string, ProjectRegisterImage>()
  origin.forEach((item) => {
    if (item.filePath) map.set(item.filePath, item)
  })
  incoming.forEach((item) => {
    if (!item.filePath) return
    const current = map.get(item.filePath)
    map.set(item.filePath, { ...item, remark: current?.remark || '' })
  })
  return Array.from(map.values())
}

const toSourceFiles = (files: FileList | File[]) => Array.from(files).map((file) => ({
  file,
  relativePath: file.name
}))

const imageValidationError = (files: UploadSourceFile[]) => {
  const nonImage = files.find((item) => !item.file.type.startsWith('image/'))
  if (nonImage) return `只能上传图片：${nonImage.file.name}`
  const tooLarge = files.find((item) => item.file.size > maxImageSize)
  if (tooLarge) return `单张图片不能大于50M：${tooLarge.file.name}`
  return ''
}

export function ImageAttachmentField({
  inputId,
  value,
  onChange,
  disabled = false,
  uploadDisabled = disabled,
  projectId,
  registerId,
  canGenerateDescription = false
}: ImageAttachmentFieldProps) {
  const [uploading, setUploading] = useState(false)
  const [dragging, setDragging] = useState(false)
  const [error, setError] = useState('')
  const [previewImage, setPreviewImage] = useState<ProjectRegisterImage | null>(null)
  const [generatingPath, setGeneratingPath] = useState('')
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const uploadBlocked = disabled || uploadDisabled

  const uploadImages = async (files: UploadSourceFile[]) => {
    if (uploadBlocked || files.length === 0) return
    const validationError = imageValidationError(files)
    if (validationError) {
      setError(validationError)
      return
    }
    try {
      setUploading(true)
      setError('')
      const attachments = await uploadAttachments(files)
      onChange(mergeImages(value, attachments))
    } catch (uploadError) {
      setError(readApiError(uploadError, '上传图片失败'))
    } finally {
      setUploading(false)
    }
  }

  const handleFileInputChange = async (event: ChangeEvent<HTMLInputElement>) => {
    const files = event.target.files
    if (!files || files.length === 0) return
    await uploadImages(toSourceFiles(files))
    event.target.value = ''
  }

  const removeImage = (target: ProjectRegisterImage) => {
    if (disabled) return
    onChange(value.filter((item) => item.filePath !== target.filePath))
  }

  const updateImageRemark = (target: ProjectRegisterImage, remark: string) => {
    if (disabled) return
    onChange(value.map((item) => item.filePath === target.filePath ? { ...item, remark } : item))
  }

  const generateImageRemark = async (target: ProjectRegisterImage) => {
    if (disabled || !canGenerateDescription) return
    if (!projectId && !registerId) {
      setError('请先选择项目，再生成图片说明')
      return
    }
    try {
      setGeneratingPath(target.filePath)
      setError('')
      const res = await api.post<{ remark: string }>('/ai/register-image-description', {
        projectId: projectId ? Number(projectId) : undefined,
        registerId,
        image: target
      }, { timeout: 60000 })
      const remark = (res.data.remark || '').trim()
      if (remark) {
        updateImageRemark(target, remark)
      }
    } catch (generateError) {
      setError(readApiError(generateError, 'AI 图片说明生成失败'))
    } finally {
      setGeneratingPath('')
    }
  }

  const handleDrop = (event: DragEvent<HTMLDivElement>) => {
    event.preventDefault()
    setDragging(false)
    if (uploadBlocked) return
    void uploadImages(toSourceFiles(event.dataTransfer.files))
  }

  return (
    <div className="image-attachment-field">
      <input id={inputId} ref={fileInputRef} className="sr-only" type="file" accept="image/*" multiple onChange={(event) => { void handleFileInputChange(event) }} disabled={uploadBlocked || uploading} />
      <div className="image-attachment-toolbar">
        <button type="button" className="btn secondary image-upload-btn" onClick={() => fileInputRef.current?.click()} disabled={uploadBlocked || uploading}>
          <ImagePlus size={16} aria-hidden="true" />
          {uploading ? '上传中...' : '上传图片'}
        </button>
        <span>支持多选和拖放，单张不超过50M</span>
      </div>
      <div
        className={`image-dropzone${dragging ? ' active' : ''}`}
        onDragEnter={(event) => { event.preventDefault(); if (!uploadBlocked) setDragging(true) }}
        onDragOver={(event) => { event.preventDefault(); if (!uploadBlocked) setDragging(true) }}
        onDragLeave={(event) => { event.preventDefault(); setDragging(false) }}
        onDrop={handleDrop}
      >
        <UploadCloud size={18} aria-hidden="true" />
        <span>{uploadDisabled ? '当前账号无上传权限' : (disabled ? '当前不可上传' : (uploading ? '正在上传，请稍候...' : '拖放图片到这里'))}</span>
      </div>
      {value.length > 0 && (
        <div className="image-attachment-grid">
          {value.map((item) => (
            <figure key={item.filePath} className="image-attachment-item">
              <button type="button" className="image-preview-thumb" onClick={() => setPreviewImage(item)} aria-label={`预览${item.relativePath || item.fileName || '图片'}`}>
                <img src={item.filePath} alt={item.relativePath || item.fileName || '登记项图片'} />
              </button>
              <figcaption>
                <span>{item.relativePath || item.fileName || '图片'}</span>
                <small>{formatFileSize(item.fileSize)}</small>
              </figcaption>
              <textarea
                className="image-remark-input"
                rows={2}
                maxLength={500}
                value={item.remark || ''}
                placeholder="图片备注"
                onChange={(event) => updateImageRemark(item, event.target.value)}
                disabled={disabled}
                aria-label={`${item.relativePath || item.fileName || '图片'}备注`}
              />
              {canGenerateDescription && (
                <button
                  type="button"
                  className="btn secondary image-ai-btn"
                  onClick={() => { void generateImageRemark(item) }}
                  disabled={disabled || generatingPath === item.filePath || (!projectId && !registerId)}
                >
                  <Sparkles size={14} aria-hidden="true" />
                  {generatingPath === item.filePath ? '生成中...' : 'AI生成'}
                </button>
              )}
              <button type="button" className="image-remove-btn" onClick={() => removeImage(item)} disabled={disabled} aria-label={`移除${item.relativePath || item.fileName || '图片'}`}>
                <Trash2 size={14} aria-hidden="true" />
              </button>
            </figure>
          ))}
        </div>
      )}
      {error && <p className="error">{error}</p>}
      <ImagePreviewOverlay image={previewImage} onClose={() => setPreviewImage(null)} />
    </div>
  )
}
