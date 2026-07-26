import type { CSSProperties } from 'react'

interface SkeletonProps {
  className?: string
  width?: string | number
  height?: string | number
  variant?: 'text' | 'title' | 'circle' | 'card'
  style?: CSSProperties
}

export default function Skeleton({ className = '', width, height, variant = 'text', style }: SkeletonProps) {
  const variantClass = `skeleton-${variant}`
  const computedStyle: CSSProperties = { ...style }
  if (width) computedStyle.width = typeof width === 'number' ? `${width}px` : width
  if (height) computedStyle.height = typeof height === 'number' ? `${height}px` : height

  return (
    <div
      className={`skeleton ${variantClass} ${className}`}
      style={computedStyle}
      aria-hidden="true"
    />
  )
}

export function MetricCardSkeleton() {
  return (
    <div className="metric-card">
      <Skeleton variant="circle" width={36} height={36} className="metric-card-icon" />
      <Skeleton variant="title" width="40%" style={{ marginTop: '12px' }} />
      <Skeleton variant="text" width="60%" style={{ marginTop: '8px' }} />
    </div>
  )
}

export function ListItemSkeleton() {
  return (
    <div className="list-item" style={{ pointerEvents: 'none' }}>
      <div className="list-item-content">
        <Skeleton variant="circle" width={36} height={36} />
        <div className="list-item-info">
          <Skeleton variant="text" width={120} />
          <Skeleton variant="text" width={180} style={{ marginTop: '4px' }} />
        </div>
      </div>
      <Skeleton variant="text" width={60} height={24} />
    </div>
  )
}
