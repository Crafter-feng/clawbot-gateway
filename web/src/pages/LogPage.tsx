import { useEffect, useState, useCallback, useRef } from 'react'
import { api } from '../api/client'
import Button from '../components/ui/Button'
import Select from '../components/ui/Select'
import { ListItemSkeleton } from '../components/ui/Skeleton'

interface LogEntry {
  time: string
  level: string
  message: string
  source?: string
  attrs?: Record<string, string | number | boolean>
}

// 分类定义：大类 → 子类映射
const CATEGORIES: Record<string, { label: string; subFilter: { label: string; cmp: string }[] }> = {
  '': { label: '全部', subFilter: [] },
  api: {
    label: 'API',
    subFilter: [
      { label: '全部', cmp: '' },
      { label: '管理 API', cmp: 'api' },
    ],
  },
  bot: {
    label: '机器人',
    subFilter: [
      { label: '全部', cmp: '' },
      { label: '连接器', cmp: 'bot' },
    ],
  },
  pipeline: {
    label: '管道',
    subFilter: [
      { label: '全部', cmp: '' },
      { label: '消息处理', cmp: 'pipeline' },
      { label: '命令解析', cmp: 'command' },
    ],
  },
  ilink: {
    label: 'iLink',
    subFilter: [
      { label: '全部', cmp: '' },
      { label: '服务端', cmp: 'ilink' },
    ],
  },
  backend: {
    label: '后端',
    subFilter: [
      { label: '全部', cmp: '' },
      { label: 'Echo', cmp: 'echo' },
    ],
  },
  system: {
    label: '系统',
    subFilter: [
      { label: '全部', cmp: '' },
      { label: '主进程', cmp: 'main' },
    ],
  },
}

export default function LogPage() {
  const [entries, setEntries] = useState<LogEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [category, setCategory] = useState('')
  const [subCmp, setSubCmp] = useState('')
  const [backends, setBackends] = useState<string[]>([])
  const [backendId, setBackendId] = useState('')
  const [limit, setLimit] = useState('100')
  const [autoRefresh, setAutoRefresh] = useState(true)
  const timerRef = useRef<ReturnType<typeof setInterval>>(undefined)

  const fetchLogs = useCallback(async () => {
    try {
      const params = new URLSearchParams()
      if (limit) params.set('limit', limit)
      if (subCmp) params.set('component', subCmp)
      if (backendId) params.set('backend', backendId)
      const res = await api.get<{ entries: LogEntry[] }>(`/api/v1/logs?${params}`)
      setEntries(res.entries || [])
    } catch {
      // silent
    } finally {
      setLoading(false)
    }
  }, [subCmp, backendId, limit])

  const fetchCategories = useCallback(async () => {
    try {
      const res = await api.get<{ backends: string[] }>('/api/v1/logs/categories')
      setBackends(res.backends || [])
    } catch {
      // silent
    }
  }, [])

  useEffect(() => {
    fetchLogs()
    fetchCategories()
  }, [fetchLogs, fetchCategories])

  useEffect(() => {
    if (autoRefresh) {
      timerRef.current = setInterval(fetchLogs, 5000)
      return () => clearInterval(timerRef.current)
    } else {
      if (timerRef.current) clearInterval(timerRef.current)
    }
  }, [autoRefresh, fetchLogs])

  // 切换分类时重置子筛选
  const handleCategoryChange = (cat: string) => {
    setCategory(cat)
    setSubCmp('')
    setBackendId('')
  }

  const catInfo = CATEGORIES[category] || CATEGORIES['']
  const showSubCmp = category === 'backend'
    ? backends.map(b => ({ label: b, cmp: b }))
    : catInfo.subFilter

  const levelColors: Record<string, string> = {
    DEBUG: 'var(--text-muted)',
    INFO: 'var(--accent)',
    WARN: 'var(--warning)',
    ERROR: 'var(--danger)',
  }

  const getBackendInfo = (entry: LogEntry): string | null => {
    if (entry.attrs?.cmp) {
      const cmp = entry.attrs.cmp as string
      if (entry.attrs?.backend) return `${cmp} > ${entry.attrs.backend}`
      return cmp
    }
    return null
  }

  return (
    <div>
      <div className="page-header">
        <h1>日志</h1>
        <p>系统运行日志</p>
      </div>

      <div className="dashboard-content">
        <div className="card">
          <div className="card-header">
            <h2 className="card-title">日志查询</h2>
          </div>
          <div style={{ display: 'flex', gap: 'var(--space-3)', alignItems: 'flex-end', flexWrap: 'wrap' }}>
            {/* 分类选择 */}
            <div className="segmented-control" style={{ display: 'flex', gap: 0, border: '1px solid var(--border)', borderRadius: 'var(--radius)', overflow: 'hidden' }}>
              {Object.entries(CATEGORIES).map(([key, val]) => (
                <button
                  key={key}
                  onClick={() => handleCategoryChange(key)}
                  style={{
                    padding: 'var(--space-1) var(--space-3)',
                    border: 'none',
                    background: category === key ? 'var(--accent)' : 'transparent',
                    color: category === key ? 'white' : 'var(--text)',
                    cursor: 'pointer',
                    fontSize: 'var(--font-size-sm)',
                    fontWeight: category === key ? 600 : 400,
                  }}
                >
                  {val.label}
                </button>
              ))}
            </div>

            {/* 子类 / 后端筛选 */}
            {category === 'backend' ? (
              <Select
                label="后端"
                value={backendId}
                onChange={(e) => setBackendId(e.target.value)}
                style={{ minWidth: 140 }}
              >
                <option value="">全部后端</option>
                {backends.map((b) => (
                  <option key={b} value={b}>{b}</option>
                ))}
              </Select>
            ) : showSubCmp.length > 0 ? (
              <Select
                label="子类"
                value={subCmp}
                onChange={(e) => setSubCmp(e.target.value)}
                style={{ minWidth: 140 }}
              >
                {showSubCmp.map((s) => (
                  <option key={s.cmp} value={s.cmp}>{s.label}</option>
                ))}
              </Select>
            ) : null}

            <Select
              label="条数"
              value={limit}
              onChange={(e) => setLimit(e.target.value)}
              style={{ minWidth: 100 }}
            >
              <option value="50">50</option>
              <option value="100">100</option>
              <option value="200">200</option>
              <option value="500">500</option>
            </Select>

            <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)', paddingBottom: 'var(--space-1)' }}>
              <label style={{ fontSize: 'var(--font-size-sm)', color: 'var(--text-muted)' }}>
                <input
                  type="checkbox"
                  checked={autoRefresh}
                  onChange={(e) => setAutoRefresh(e.target.checked)}
                  style={{ marginRight: 'var(--space-1)' }}
                />
                自动刷新
              </label>
            </div>
            <Button onClick={fetchLogs} variant="secondary" size="sm">
              刷新
            </Button>
          </div>
        </div>

        <div className="card" style={{ overflow: 'hidden' }}>
          <div
            className="code-block"
            style={{
              maxHeight: '70vh',
              overflowY: 'auto',
              fontSize: 'var(--font-size-xs)',
              fontFamily: 'var(--font-mono)',
              lineHeight: 1.6,
              padding: 0,
              background: 'var(--surface-dim)',
            }}
          >
            {loading ? (
              <div style={{ padding: 'var(--space-3)' }}>
                <ListItemSkeleton />
              </div>
            ) : entries.length === 0 ? (
              <div style={{ padding: 'var(--space-4)', textAlign: 'center', color: 'var(--text-muted)' }}>
                暂无日志
              </div>
            ) : (
              entries.map((entry, i) => (
                <div
                  key={i}
                  style={{
                    padding: 'var(--space-1) var(--space-3)',
                    borderBottom: '1px solid var(--border)',
                    display: 'flex',
                    gap: 'var(--space-2)',
                    alignItems: 'flex-start',
                  }}
                >
                  <span style={{ color: 'var(--text-muted)', whiteSpace: 'nowrap', minWidth: '10ch' }}>
                    {entry.time ? entry.time.slice(11, 19) : ''}
                  </span>
                  <span
                    style={{
                      color: levelColors[entry.level] || 'var(--text)',
                      fontWeight: 600,
                      minWidth: '4ch',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    {entry.level}
                  </span>
                  <span style={{ color: 'var(--text-muted)', minWidth: '12ch', whiteSpace: 'nowrap' }}>
                    {getBackendInfo(entry) || ''}
                  </span>
                  <span style={{ flex: 1, wordBreak: 'break-all', color: 'var(--text)' }}>
                    {entry.message}
                    {entry.attrs?.error && (
                      <span style={{ color: 'var(--danger)', marginLeft: 'var(--space-1)' }}>
                        ({String(entry.attrs.error)})
                      </span>
                    )}
                  </span>
                </div>
              ))
            )}
          </div>
        </div>
      </div>
    </div>
  )
}