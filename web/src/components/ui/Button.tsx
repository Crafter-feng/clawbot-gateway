import { type ButtonHTMLAttributes, type ReactNode, forwardRef } from 'react'

type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'ghost' | 'ghost-danger'
type ButtonSize = 'sm' | 'md' | 'lg'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
  loading?: boolean
  icon?: ReactNode
}

const variantClass: Record<ButtonVariant, string> = {
  primary: 'btn-primary',
  secondary: 'btn-secondary',
  danger: 'btn-danger',
  ghost: 'btn-ghost',
  'ghost-danger': 'btn-ghost-danger',
}

const sizeClass: Record<ButtonSize, string> = {
  sm: 'btn-sm',
  md: 'btn-md',
  lg: 'btn-lg',
}

const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = 'primary', size = 'md', loading, icon, children, className = '', disabled, type, ...props }, ref) => {
    return (
      <button
        ref={ref}
        className={`btn ${variantClass[variant]} ${sizeClass[size]} ${className}`}
        disabled={disabled || loading}
        type={type || 'button'}
        {...props}
      >
        {loading ? <span className="spinner spinner-sm spinner-white" /> : icon}
        {children}
      </button>
    )
  }
)

Button.displayName = 'Button'

export default Button
