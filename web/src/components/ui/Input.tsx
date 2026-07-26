import { type InputHTMLAttributes, forwardRef } from 'react'

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string
  hint?: string
  error?: string
}

const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ label, hint, error, className = '', id, ...props }, ref) => {
    const inputId = id || (label ? label.toLowerCase().replace(/\s+/g, '-') : undefined)

    return (
      <div className="input-group">
        {label && (
          <label className="input-label" htmlFor={inputId}>
            {label}
          </label>
        )}
        <input
          ref={ref}
          id={inputId}
          className={`input ${error ? 'input-error-state' : ''} ${className}`}
          aria-describedby={hint ? `${inputId}-hint` : error ? `${inputId}-error` : undefined}
          aria-invalid={!!error}
          {...props}
        />
        {hint && !error && (
          <span className="input-hint" id={`${inputId}-hint`}>
            {hint}
          </span>
        )}
        {error && (
          <span className="input-error" id={`${inputId}-error`} role="alert">
            {error}
          </span>
        )}
      </div>
    )
  }
)

Input.displayName = 'Input'

export default Input
