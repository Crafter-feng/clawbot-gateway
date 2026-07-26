import type { CSSProperties, ReactNode } from 'react'

type TagVariant = 'success' | 'danger' | 'warning' | 'info' | 'neutral'

interface TagProps {
  variant?: TagVariant
  children: ReactNode
  className?: string
  style?: CSSProperties
}

const variantClass: Record<TagVariant, string> = {
  success: 'tag-success',
  danger: 'tag-danger',
  warning: 'tag-warning',
  info: 'tag-info',
  neutral: 'tag-neutral',
}

export default function Tag({ variant = 'neutral', children, className = '', style }: TagProps) {
  return (
    <span className={`tag ${variantClass[variant]} ${className}`} style={style}>
      {children}
    </span>
  )
}
