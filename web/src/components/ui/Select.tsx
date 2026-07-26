import { type SelectHTMLAttributes, forwardRef } from 'react'

interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  label?: string
  hint?: string
  error?: string
}

const Select = forwardRef<HTMLSelectElement, SelectProps>(
  ({ label, hint, error, className = '', id, children, ...props }, ref) => {
    const selectId = id || (label ? label.toLowerCase().replace(/\s+/g, '-') : undefined)

    return (
      <div className="input-group">
        {label && (
          <label className="input-label" htmlFor={selectId}>
            {label}
          </label>
        )}
        <select
          ref={ref}
          id={selectId}
          className={`select ${error ? 'input-error-state' : ''} ${className}`}
          aria-describedby={error ? `${selectId}-error` : hint ? `${selectId}-hint` : undefined}
          aria-invalid={!!error}
          {...props}
        >
          {children}
        </select>
        {hint && !error && (
          <span className="input-hint" id={`${selectId}-hint`}>
            {hint}
          </span>
        )}
        {error && (
          <span className="input-error" id={`${selectId}-error`} role="alert">
            {error}
          </span>
        )}
      </div>
    )
  }
)

Select.displayName = 'Select'

export default Select
