import { useEffect, useState, useRef, useCallback } from 'react'
import { api } from '../api/client'
import Button from '../components/ui/Button'

interface LogEntry {
  time: string
  level: string
  message: string
  count?: number
  source?: string
  attrs?: Record<string, string | number | boolean>
}

const CATEGORIES: { key: string; label: string; icon: string }[] = [
  { key: '', label: '全部', icon: '◉' },
  { key: 'api', label: 'HTTP', icon: '⇄' },
  { key: 'bot', label: '连接', icon: '◈' },
  { key: 'pipeline', label: '管道', icon: '⇝' },
  { key: 'ilink', label: 'iLink', icon: '◎' },
]
const CMP_MAP: Record<string, { label: string; cmp: string }[]> = {
  api: [{ label: 'HTTP 请求', cmp: 'api' }],
  bot: [
    { label: '连接器', cmp: 'bot' },
    { label: '扫码登录', cmp: 'qr' },
  ],
  pipeline: [
    { label: '消息处理', cmp: 'pipeline' },
    { label: '命令解析', cmp: 'command' },
  ],
  ilink: [{ label: '服务端', cmp: 'ilink' }],
}

const LEVEL_META: Record<string, { color: string; label: string }> = {
  DEBUG: { color: '#6b7280', label: 'DBG' },
  INFO: { color: '#22c55e', label: 'INF' },
  WARN: { color: '#eab308', label: 'WRN' },
  ERROR: { color: '#ef4444', label: 'ERR' },
}
export default function LogPage() {
  const [entries, setEntries] = useState<LogEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [category, setCategory] = useState('')
  const [subCmp, setSubCmp] = useState('')
  const [limit, setLimit] = useState('100')
  const listRef = useRef<HTMLDivElement>(null)

  // 当前选中分类对应的 component 筛选值
  const activeComponent = category ? (subCmp || category) : subCmp

  const fetchLogs = useCallback(async () => {
    try {
      setError('')
      const params = new URLSearchParams()
      if (limit) params.set('limit', limit)
      if (activeComponent) params.set('component', activeComponent)
      const res = await api.get<{ entries: LogEntry[] }>(`/api/v1/logs?${params}`)
      setEntries(res.entries || [])
    } catch {
      setError('加载日志失败')
    } finally {
      setLoading(false)
    }
  }, [activeComponent, limit])

  useEffect(() => { fetchLogs() }, [fetchLogs])

  const handleCategory = (key: string) => {
    setCategory(key)
    setSubCmp('')
    setLoading(true)
  }
  const showSubCmp = CMP_MAP[category] || []
  const getCmp = (e: LogEntry): string => {
    const c = e.attrs?.cmp as string | undefined
    if (!c) return ''
    for (const items of Object.values(CMP_MAP)) {
      for (const item of items) {
        if (item.cmp === c) return item.label
      }
    }
    return c
  }

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      {/* Header */}
      <div className="page-header" style={{ flexShrink: 0 }}>
        <h1>日志</h1>
        <p>系统运行日志 · 实时监控</p>
      </div>

      {/* Filter bar */}
      <div style={{
        flexShrink: 0,
        margin: '0 var(--space-4) var(--space-3)',
        background: 'var(--surface)',
        borderRadius: 'var(--radius)',
        border: '1px solid var(--border)',
        overflow: 'hidden',
      }}>
        {/* Category tabs */}
        <div style={{
          display: 'flex',
          gap: 0,
          borderBottom: '1px solid var(--border)',
          background: 'var(--surface-dim)',
          padding: '0 var(--space-2)',
        }}>
          {CATEGORIES.map(c => (
            <button
              key={c.key}
              onClick={() => handleCategory(c.key)}
              style={{
                flex: 1,
                padding: '10px 8px',
                border: 'none',
                borderBottom: category === c.key ? '2px solid var(--accent)' : '2px solid transparent',
                background: 'transparent',
                color: category === c.key ? 'var(--accent)' : 'var(--text-muted)',
                cursor: 'pointer',
                fontSize: '12px',
                fontWeight: category === c.key ? 600 : 400,
                transition: 'color .15s, border-color .15s',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                gap: '6px',
              }}
            >
              <span style={{ fontSize: '14px' }}>{c.icon}</span>
              <span>{c.label}</span>
            </button>
          ))}
        </div>

        {/* Sub filters — single row */}
        <div style={{
          display: 'flex',
          gap: 'var(--space-3)',
          alignItems: 'center',
          padding: 'var(--space-3)',
        }}>
          {/* Left: sub-category */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <label style={{ fontSize: '12px', color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>
              子类
            </label>
            <select
              value={subCmp}
              onChange={e => { setSubCmp(e.target.value); setLoading(true) }}
              style={{
                fontSize: '12px',
                padding: '4px 8px',
                borderRadius: 'var(--radius)',
                border: '1px solid var(--border)',
                background: 'var(--surface)',
                color: 'var(--text)',
                minWidth: 120,
              }}
            >
              <option value="">全部</option>
              {showSubCmp.map(s => (
                <option key={s.cmp} value={s.cmp}>{s.label}</option>
              ))}
            </select>
          </div>
          {/* Spacer */}
          <div style={{ flex: 1 }} />

          {/* Right: count + refresh */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
              <label style={{ fontSize: '12px', color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>条数</label>
              <select
                value={limit}
                onChange={e => setLimit(e.target.value)}
                style={{
                  fontSize: '12px',
                  padding: '4px 8px',
                  borderRadius: 'var(--radius)',
                  border: '1px solid var(--border)',
                  background: 'var(--surface)',
                  color: 'var(--text)',
                  minWidth: 65,
                }}
              >
                {[50, 100, 200, 500].map(n => (
                  <option key={n} value={String(n)}>{n}</option>
                ))}
              </select>
            </div>

            <Button
              onClick={() => { setLoading(true); fetchLogs() }}
              variant="secondary"
              size="sm"
            >
              刷新
            </Button>
          </div>
        </div>
      </div>

      {/* Log table */}
      <div style={{
        flex: 1,
        margin: '0 var(--space-4) var(--space-4)',
        background: 'var(--surface)',
        borderRadius: 'var(--radius)',
        border: '1px solid var(--border)',
        overflow: 'hidden',
        display: 'flex',
        flexDirection: 'column',
      }}>
        {/* Table header */}
        <div style={{
          display: 'flex',
          gap: 0,
          borderBottom: '1px solid var(--border)',
          background: 'var(--surface-dim)',
          fontSize: '11px',
          fontWeight: 600,
          color: 'var(--text-muted)',
          textTransform: 'uppercase',
          letterSpacing: '0.05em',
          flexShrink: 0,
        }}>
          <div style={{ width: 72, padding: '10px 12px' }}>时间</div>
          <div style={{ width: 52, padding: '10px 12px' }}>级别</div>
          <div style={{ width: 160, padding: '10px 12px' }}>组件</div>
          <div style={{ flex: 1, padding: '10px 12px' }}>消息</div>
        </div>

        {/* Table body */}
        <div ref={listRef} style={{
          flex: 1,
          overflowY: 'auto',
          fontFamily: "'JetBrains Mono', ui-monospace, 'SF Mono', monospace",
          fontSize: '12px',
          lineHeight: 1.6,
        }}>
          {loading ? (
            <div style={{ padding: '32px', textAlign: 'center', color: 'var(--text-muted)' }}>
              <div style={{
                display: 'inline-block',
                width: 20,
                height: 20,
                border: '2px solid var(--border)',
                borderTopColor: 'var(--accent)',
                borderRadius: '50%',
                animation: 'spin .6s linear infinite',
              }} />
            </div>
          ) : error ? (
            <div style={{ padding: '48px 24px', textAlign: 'center', color: 'var(--text-muted)' }}>
              <div style={{ fontSize: '24px', marginBottom: '8px', opacity: 0.3 }}>⚠</div>
              <div style={{ fontSize: '13px', color: '#ef4444' }}>{error}</div>
              <Button onClick={() => { setLoading(true); fetchLogs() }} variant="secondary" size="sm" style={{ marginTop: 12 }}>
                重试
              </Button>
            </div>
          ) : entries.length === 0 ? (
            <div style={{ padding: '48px 24px', textAlign: 'center', color: 'var(--text-muted)' }}>
              <div style={{ fontSize: '24px', marginBottom: '8px', opacity: 0.3 }}>⏚</div>
              <div style={{ fontSize: '13px' }}>暂无日志</div>
            </div>
          ) : (
            entries.map((entry, i) => {
              const meta = LEVEL_META[entry.level] || { color: 'var(--text)', label: entry.level }
              const cmp = getCmp(entry)
              return (
                <div
                  key={i}
                  style={{
                    display: 'flex',
                    gap: 0,
                    borderBottom: '1px solid var(--border)',
                    background: entry.level === 'ERROR' ? 'rgba(239,68,68,0.04)' : (i % 2 === 1 ? 'rgba(128,128,128,0.02)' : 'transparent'),
                    transition: 'background .15s',
                    animation: i < 10 ? `fadeIn .2s ease-out ${i * 20}ms both` : undefined,
                  }}
                  onMouseEnter={e => (e.currentTarget.style.background = 'rgba(128,128,128,0.06)')}
                  onMouseLeave={e => {
                    e.currentTarget.style.background = entry.level === 'ERROR'
                      ? 'rgba(239,68,68,0.04)'
                      : (i % 2 === 1 ? 'rgba(128,128,128,0.02)' : 'transparent')
                  }}
                >
                  <div style={{ width: 72, padding: '4px 12px', color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>
                    {entry.time ? entry.time.slice(11, 19) : ''}
                  </div>
                  <div style={{ width: 52, padding: '4px 12px' }}>
                    <span style={{
                      display: 'inline-block',
                      padding: '0 6px',
                      fontSize: '10px',
                      fontWeight: 700,
                      lineHeight: '18px',
                      borderRadius: 3,
                      background: `${meta.color}18`,
                      color: meta.color,
                      letterSpacing: '0.03em',
                    }}>
                      {meta.label}
                    </span>
                  </div>
                  <div style={{
                    width: 160,
                    padding: '4px 12px',
                    color: 'var(--text-muted)',
                    whiteSpace: 'nowrap',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                  }}>
                    {cmp}
                  </div>
                  <div style={{ flex: 1, padding: '4px 12px', color: 'var(--text)', wordBreak: 'break-all', display: 'flex', alignItems: 'center', gap: 6 }}>
                    <span style={{ flex: 1 }}>{entry.message}</span>
                    {entry.count && entry.count > 1 && (
                      <span style={{
                        fontSize: '10px',
                        fontWeight: 700,
                        lineHeight: '16px',
                        padding: '0 5px',
                        borderRadius: 8,
                        background: 'var(--surface-dim)',
                        color: 'var(--text-muted)',
                        flexShrink: 0,
                      }}>
                        ×{entry.count}
                      </span>
                    )}
                    {entry.attrs?.error && (
                      <span style={{ color: '#ef4444', marginLeft: 6 }}>({String(entry.attrs.error)})</span>
                    )}
                  </div>
                </div>
              )
            })
          )}
        </div>
      </div>

      {/* Keyframes injection */}
      <style>{`
        @keyframes fadeIn { from { opacity: 0; transform: translateY(4px); } to { opacity: 1; transform: translateY(0); } }
        @keyframes spin { to { transform: rotate(360deg); } }
      `}</style>
    </div>
  )
}