import { InputHTMLAttributes } from 'react'
import { Search, X } from 'lucide-react'

interface SearchFieldProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'onChange' | 'type' | 'value'> {
  value: string
  onChange: (value: string) => void
  onClear?: () => void
}

export function SearchField({ value, onChange, onClear, className = '', ...props }: SearchFieldProps) {
  return (
    <div className={`search-field${className ? ` ${className}` : ''}`}>
      <Search className="search-field-icon" size={16} aria-hidden="true" />
      <input
        {...props}
        type="search"
        className="search-field-input"
        value={value}
        inputMode={props.inputMode || 'search'}
        enterKeyHint={props.enterKeyHint || 'search'}
        autoComplete={props.autoComplete || 'off'}
        spellCheck={props.spellCheck ?? false}
        onChange={(event) => onChange(event.target.value)}
      />
      {value && (
        <button
          type="button"
          className="search-field-clear"
          aria-label="清空搜索"
          onClick={() => {
            onChange('')
            onClear?.()
          }}
        >
          <X size={14} aria-hidden="true" />
        </button>
      )}
    </div>
  )
}
