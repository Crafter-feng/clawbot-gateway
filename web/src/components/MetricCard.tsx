interface Props {
  label: string
  value: string | number
}

export default function MetricCard({ label, value }: Props) {
  return (
    <div className="metric-card" style={{
      background: 'var(--bg-card)',
      border: '1px solid var(--border)',
      borderRadius: 'var(--radius-lg)',
      padding: '20px',
    }}>
      <div style={{ fontSize: '28px', fontWeight: 700, letterSpacing: '-0.02em', lineHeight: 1.2 }}>
        {value}
      </div>
      <div style={{ marginTop: '6px', color: 'var(--text-secondary)', fontSize: '13px', fontWeight: 500 }}>
        {label}
      </div>
    </div>
  )
}