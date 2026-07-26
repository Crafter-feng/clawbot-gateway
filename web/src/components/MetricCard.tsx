import type { ReactNode } from 'react'

interface Props {
  label: string
  value: string | number
  icon?: ReactNode
  trend?: {
    value: number
    label?: string
  }
}

export default function MetricCard({ label, value, icon, trend }: Props) {
  return (
    <div className="metric-card">
      {icon && (
        <div className="metric-card-icon">
          {icon}
        </div>
      )}
      <div className="metric-card-value">{value}</div>
      <div className="metric-card-label">{label}</div>
      {trend && (
        <div
          className="text-xs font-medium"
          style={{
            marginTop: '4px',
            color: trend.value >= 0 ? 'var(--success)' : 'var(--danger)',
          }}
        >
          {trend.value >= 0 ? '↑' : '↓'} {Math.abs(trend.value)}
          {trend.label && ` ${trend.label}`}
        </div>
      )}
    </div>
  )
}
