import { type TextareaHTMLAttributes, forwardRef } from 'react'

interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  label?: string
  hint?: string
  error?: string
}

const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(
  ({ label, hint, error, className = '', id, ...props }, ref) => {
    const textareaId = id || (label ? label.toLowerCase().replace(/\s+/g, '-') : undefined)

    return (
      <div className="input-group">
        {label && (
          <label className="input-label" htmlFor={textareaId}>
            {label}
          </label>
        )}
        <textarea
          ref={ref}
          id={textareaId}
          className={`input ${error ? 'input-error-state' : ''} ${className}`}
          aria-describedby={hint ? `${textareaId}-hint` : error ? `${textareaId}-error` : undefined}
          aria-invalid={!!error}
          {...props}
        />
        {hint && !error && (
          <span className="input-hint" id={`${textareaId}-hint`}>
            {hint}
          </span>
        )}
        {error && (
          <span className="input-error" id={`${textareaId}-error`} role="alert">
            {error}
          </span>
        )}
      </div>
    )
  }
)

Textarea.displayName = 'Textarea'

export default Textarea
