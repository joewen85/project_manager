import { Calendar } from 'lucide-react'
import { useRef } from 'react'

const padNumber = (value: number) => String(value).padStart(2, '0')

const formatDateTimeLocalValue = (value: Date) => {
  const year = value.getFullYear()
  const month = padNumber(value.getMonth() + 1)
  const day = padNumber(value.getDate())
  const hours = padNumber(value.getHours())
  const minutes = padNumber(value.getMinutes())
  return `${year}-${month}-${day}T${hours}:${minutes}`
}

interface DateTimeQuickFieldProps {
  inputId: string
  value: string
  disabled?: boolean
  onChange: (value: string) => void
}

export function DateTimeQuickField({ inputId, value, disabled = false, onChange }: DateTimeQuickFieldProps) {
  const inputRef = useRef<HTMLInputElement | null>(null)
  const applyNow = () => onChange(formatDateTimeLocalValue(new Date()))

  const applyTomorrow = () => {
    const next = new Date()
    next.setDate(next.getDate() + 1)
    onChange(formatDateTimeLocalValue(next))
  }

  const openPicker = () => {
    if (disabled) return
    inputRef.current?.focus()
    inputRef.current?.showPicker?.()
  }

  return (
    <div className="datetime-quick-field">
      <input ref={inputRef} id={inputId} type="datetime-local" value={value} onChange={(event) => onChange(event.target.value)} disabled={disabled} />
      <div className="datetime-quick-actions" aria-hidden={disabled}>
        <button
          type="button"
          className="datetime-quick-text-btn"
          onClick={applyNow}
          disabled={disabled}
          aria-label="开始/结束时间设为当前时刻"
          title="今天"
        >
          今天
        </button>
        <button
          type="button"
          className="datetime-quick-text-btn"
          onClick={applyTomorrow}
          disabled={disabled}
          aria-label="开始/结束时间设为明天同一时刻"
          title="明天"
        >
          明天
        </button>
      </div>
      <button
        type="button"
        className="datetime-quick-picker-btn"
        onClick={openPicker}
        disabled={disabled}
        aria-label="打开日历时间选择器"
        title="选择日期时间"
      >
        <Calendar size={16} />
      </button>
    </div>
  )
}
