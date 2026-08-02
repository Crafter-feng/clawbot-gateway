import { useEffect, useState, useRef, useCallback, useMemo } from 'react'
import { api } from '../api/client'
import EmptyState from '../components/ui/EmptyState'

interface LogEntry {
  time: string
  level: string
  message: string
  count?: number
  source?: string
  attrs?: Record<string, string | number | boolean>
}

interface Categories {
  components: string[]
  backends: string[]
}

// ── 模块分组 ──
// 已知组件 → 所属模块；新组件自动归入"其他"
const MODULE_GROUPS: Record<string, { label: string; icon: string }> = {
  main:     { label: '系统',   icon: '⚙' },
  api:      { label: 'HTTP',  icon: '⇄' },
  bot:      { label: '连接',  icon: '◈' },
  qr:       { label: '连接',  icon: '◈' },
  pipeline: { label: '管道',  icon: '⇝' },
  command:  { label: '管道',  icon: '⇝' },
  ilink:    { label: 'iLink', icon: '◎' },
  database: { label: '数据库', icon: '🗄' },
  notify:   { label: '通知',  icon: '🔔' },
}

// 模块标签 → 图标（用于 tab 显示）
const MODULE_META: Record<string, { icon: string; order: number }> = {
  '全部':   { icon: '◉', order: 0 },
  '系统':   { icon: '⚙', order: 1 },
  'HTTP':   { icon: '⇄', order: 2 },
  '连接':   { icon: '◈', order: 3 },
  '管道':   { icon: '⇝', order: 4 },
  'iLink':  { icon: '◎', order: 5 },
  '数据库': { icon: '🗄', order: 6 },
  '通知':   { icon: '🔔', order: 7 },
  '其他':   { icon: '⋯', order: 99 },
}

// ── 级别元数据 ──
const LEVEL_META: Record<string, { color: string; label: string; variant: 'danger' | 'warning' | 'info' | 'neutral' }> = {
  DEBUG: { color: '#6b7280', label: 'DBG', variant: 'neutral' },
  INFO:  { color: '#22c55e', label: 'INF', variant: 'info' },
  WARN:  { color: '#eab308', label: 'WRN', variant: 'warning' },
  ERROR: { color: '#ef4444', label: 'ERR', variant: 'danger' },
}

const LEVELS = ['', 'DEBUG', 'INFO', 'WARN', 'ERROR'] as const

// ── 辅助函数 ──

// 根据组件名获取模块标签
function cmpToModule(cmp: string): string {
  return MODULE_GROUPS[cmp]?.label || '其他'
}

// 从组件列表构建模块列表（去重+排序）
function buildModules(components: string[]): string[] {
  const seen = new Set<string>()
  const modules: string[] = []
  for (const c of components) {
    const m = cmpToModule(c)
    if (!seen.has(m)) {
      seen.add(m)
      modules.push(m)
    }
  }
  modules.sort((a, b) => (MODULE_META[a]?.order ?? 99) - (MODULE_META[b]?.order ?? 99))
  return modules
}

// 获取模块下的所有组件
function moduleComponents(module: string, allComponents: string[]): string[] {
  if (module === '全部') return allComponents
  return allComponents.filter(c => cmpToModule(c) === module)
}

// 解析 source 为短路径
function shortSource(source?: string): string {
  if (!source) return ''
  // 格式: "internal/api/handler.go:42"
  const parts = source.split('/')
  if (parts.length >= 2) return parts.slice(-2).join('/')
  return source
}

export default function LogPage() {
  const [entries, setEntries] = useState<LogEntry[]>([])
  const [categories, setCategories] = useState<Categories>({ components: [], backends: [] })
  const [sseConnected, setSseConnected] = useState(false)

  // Filters
  const [activeModule, setActiveModule] = useState('全部')
  const [activeBackend, setActiveBackend] = useState('')
  const [levelFilter, setLevelFilter] = useState('')
  const [searchQuery, setSearchQuery] = useState('')
  const [limit, setLimit] = useState(200)

  // UI state
  const [follow, setFollow] = useState(true)
  const [expandedKey, setExpandedKey] = useState<string | null>(null)
  const listRef = useRef<HTMLDivElement>(null)
  const followRef = useRef(true)
  followRef.current = follow

  const isTauri = typeof window !== 'undefined' && ('__TAURI_INTERNALS__' in window)

  // 动态模块列表
  const modules = useMemo(
    () => ['全部', ...buildModules(categories.components)],
    [categories.components],
  )

  // 当前模块下的组件列表（用于 SSE 过滤）
  const activeComponents = useMemo(
    () => moduleComponents(activeModule, categories.components),
    [activeModule, categories.components],
  )

  // ── 初始加载：分类 + 历史日志 ──
  useEffect(() => {
    api.get<Categories>('/api/v1/logs/categories')
      .then(res => setCategories(res))
      .catch(() => {})
    // 加载最近 200 条历史日志，后端返回正序（旧→新），反转成倒序（新→旧）
    api.get<{ entries: LogEntry[] }>('/api/v1/logs?limit=200')
      .then(res => setEntries(res.entries.reverse()))
      .catch(() => {})
  }, [])

  // ── SSE 实时推送 ──
  useEffect(() => {
    const proto = window.location.protocol === 'https:' ? 'https:' : 'http:'
    const host = window.location.host
    const base = isTauri ? 'http://localhost:6798' : `${proto}//${host}`
    const es = new EventSource(`${base}/api/v1/logs/stream`)

    es.onopen = () => setSseConnected(true)

    es.onmessage = (e) => {
      try {
        const entry: LogEntry = JSON.parse(e.data)
        setEntries(prev => {
          // 合并相邻相同条目
          if (prev.length > 0) {
            const last = prev[0]
            if (last.message === entry.message && last.level === entry.level) {
              prev[0] = { ...last, count: (last.count || 1) + 1, time: entry.time }
              return [...prev]
            }
          }
          return [entry, ...prev].slice(0, 500)
        })
      } catch { /* ignore parse errors */ }
    }

    es.onerror = () => { setSseConnected(false) }

    return () => { es.close(); setSseConnected(false) }
  }, [])

  // ── 自动跟随 ──
  useEffect(() => {
    if (!followRef.current || !listRef.current) return
    listRef.current.scrollTop = 0
  }, [entries])

  const handleScroll = useCallback(() => {
    const el = listRef.current
    if (!el) return
    const atTop = el.scrollTop < 10
    if (!atTop) setFollow(false)
  }, [])

  // ── 客户端过滤 ──
  const filtered = useMemo(() => {
    let result = entries

    // 模块过滤（client-side，因为 SSE 是全局的）
    if (activeModule !== '全部') {
      result = result.filter(e => {
        const cmp = e.attrs?.cmp as string | undefined
        return cmp && activeComponents.includes(cmp)
      })
    }

    // 后端过滤
    if (activeBackend) {
      result = result.filter(e => e.attrs?.backend === activeBackend)
    }

    // 级别过滤
    if (levelFilter) {
      result = result.filter(e => e.level === levelFilter)
    }

    // 搜索
    if (searchQuery) {
      const q = searchQuery.toLowerCase()
      result = result.filter(e => {
        if (e.message.toLowerCase().includes(q)) return true
        if (!e.attrs) return false
        return Object.values(e.attrs).some(v => String(v).toLowerCase().includes(q))
      })
    }

    return result.slice(0, limit)
  }, [entries, activeModule, activeComponents, activeBackend, levelFilter, searchQuery, limit])

  const getCmpLabel = (cmp: string | undefined): string => {
    const g = MODULE_GROUPS[cmp ?? '']
    return g ? `${g.icon} ${g.label}·${cmp}` : (cmp ?? '')
  }

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      {/* Header */}
      <div className="page-header" style={{ flexShrink: 0, display: 'flex', alignItems: 'center', gap: 12 }}>
        <div style={{ flex: 1 }}>
          <h1>日志</h1>
          <p style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            系统运行日志 · 实时监控
            {follow ? '· 跟随中' : ''}
            {/* SSE 状态指示 */}
            <span
              title={sseConnected ? '实时推送已连接' : '实时推送已断开'}
              style={{
                display: 'inline-block',
                width: 8,
                height: 8,
                borderRadius: '50%',
                background: sseConnected ? '#22c55e' : '#ef4444',
                boxShadow: sseConnected ? '0 0 4px rgba(34,197,94,0.5)' : 'none',
                transition: 'background .3s',
              }}
            />
            <span style={{ fontSize: 11, color: 'var(--text-muted)' }}>
              {sseConnected ? '已连接' : '已断开'}
            </span>
          </p>
        </div>
        <div style={{ fontSize: 12, color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>
          显示 {filtered.length} 条
          {entries.length > filtered.length && ` (共 ${entries.length} 条)`}
        </div>
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
        {/* Row 1: Module tabs */}
        <div style={{
          display: 'flex',
          borderBottom: '1px solid var(--border)',
          background: 'var(--surface-dim)',
          padding: '0 var(--space-2)',
          overflowX: 'auto',
        }}>
          {modules.map(m => {
            const meta = MODULE_META[m] || { icon: '⋯' }
            return (
              <button
                key={m}
                onClick={() => { setActiveModule(m); setExpandedKey(null) }}
                style={{
                  padding: '10px 14px',
                  border: 'none',
                  borderBottom: activeModule === m ? '2px solid var(--accent)' : '2px solid transparent',
                  background: 'transparent',
                  color: activeModule === m ? 'var(--accent)' : 'var(--text-muted)',
                  cursor: 'pointer',
                  fontSize: '12px',
                  fontWeight: activeModule === m ? 600 : 400,
                  whiteSpace: 'nowrap',
                  transition: 'color .15s, border-color .15s',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 5,
                }}
              >
                <span style={{ fontSize: 13 }}>{meta.icon}</span>
                {m}
              </button>
            )
          })}
        </div>

        {/* Row 2: Filters */}
        <div style={{
          display: 'flex',
          gap: 12,
          alignItems: 'center',
          padding: '10px 12px',
          flexWrap: 'wrap',
        }}>
          {/* Level filter */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 3 }}>
            {LEVELS.map(lv => {
              const active = levelFilter === lv
              const meta = lv ? LEVEL_META[lv] : { color: 'var(--text-muted)', label: 'ALL', variant: 'neutral' }
              return (
                <button
                  key={lv}
                  onClick={() => setLevelFilter(active ? '' : lv)}
                  style={{
                    padding: '3px 8px',
                    fontSize: '10px',
                    fontWeight: 700,
                    lineHeight: '16px',
                    border: `1px solid ${active ? meta.color : 'var(--border)'}`,
                    borderRadius: '4px',
                    background: active ? `${meta.color}18` : 'transparent',
                    color: active ? meta.color : 'var(--text-muted)',
                    cursor: 'pointer',
                    letterSpacing: '0.03em',
                    transition: 'all .15s',
                  }}
                >
                  {meta.label}
                </button>
              )
            })}
          </div>

          {/* Backend filter */}
          {categories.backends.length > 0 && (
            <>
              <div style={{ width: 1, height: 20, background: 'var(--border)', flexShrink: 0 }} />
              <select
                value={activeBackend}
                onChange={e => setActiveBackend(e.target.value)}
                style={{
                  fontSize: '11px',
                  padding: '3px 6px',
                  borderRadius: 'var(--radius)',
                  border: '1px solid var(--border)',
                  background: 'var(--surface)',
                  color: 'var(--text)',
                  maxWidth: 140,
                }}
                title="按后端筛选"
              >
                <option value="">全部后端</option>
                {categories.backends.map(b => (
                  <option key={b} value={b}>{b}</option>
                ))}
              </select>
            </>
          )}

          <div style={{ width: 1, height: 20, background: 'var(--border)', flexShrink: 0 }} />

          {/* Search */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 4, flex: '1 1 160px', minWidth: 120 }}>
            <span style={{ fontSize: 12, color: 'var(--text-muted)', flexShrink: 0 }}>🔍</span>
            <input
              type="text"
              placeholder="搜索消息、属性..."
              value={searchQuery}
              onChange={e => setSearchQuery(e.target.value)}
              style={{
                flex: 1,
                fontSize: '12px',
                padding: '4px 8px',
                borderRadius: 'var(--radius)',
                border: '1px solid var(--border)',
                background: 'var(--surface)',
                color: 'var(--text)',
                outline: 'none',
                minWidth: 80,
              }}
            />
          </div>

          <div style={{ flex: 1, minWidth: 0 }} />

          {/* Controls */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexShrink: 0 }}>
            {/* Follow */}
            <button
              onClick={() => setFollow(!follow)}
              title={follow ? '点击暂停跟随' : '点击恢复跟随'}
              style={{
                padding: '4px 10px',
                fontSize: '11px',
                fontWeight: 600,
                border: `1px solid ${follow ? 'var(--accent)' : 'var(--border)'}`,
                borderRadius: 'var(--radius)',
                background: follow ? 'rgba(99,102,241,0.1)' : 'transparent',
                color: follow ? 'var(--accent)' : 'var(--text-muted)',
                cursor: 'pointer',
                transition: 'all .15s',
              }}
            >
              {follow ? '⏷ 跟随' : '⏸ 暂停'}
            </button>

            {/* Limit */}
            <select
              value={limit}
              onChange={e => setLimit(Number(e.target.value))}
              style={{
                fontSize: '11px',
                padding: '4px 6px',
                borderRadius: 'var(--radius)',
                border: '1px solid var(--border)',
                background: 'var(--surface)',
                color: 'var(--text)',
              }}
            >
              {[50, 100, 200, 500].map(n => (
                <option key={n} value={n}>{n}条</option>
              ))}
            </select>
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
          borderBottom: '1px solid var(--border)',
          background: 'var(--surface-dim)',
          fontSize: '11px',
          fontWeight: 600,
          color: 'var(--text-muted)',
          textTransform: 'uppercase',
          letterSpacing: '0.05em',
          flexShrink: 0,
        }}>
          <div style={{ width: 68, padding: '10px 10px', flexShrink: 0 }}>时间</div>
          <div style={{ width: 48, padding: '10px 10px', flexShrink: 0, textAlign: 'center' }}>级别</div>
          <div style={{ width: 110, padding: '10px 10px', flexShrink: 0 }}>组件</div>
          <div style={{ flex: 1, padding: '10px 10px', minWidth: 0 }}>消息</div>
          <div style={{ width: 130, padding: '10px 10px', flexShrink: 0, display: 'none' }} className="source-col">来源</div>
          <div style={{ width: 36, padding: '10px 10px', flexShrink: 0, textAlign: 'center' }}/>
        </div>

        {/* Table body */}
        <div
          ref={listRef}
          onScroll={handleScroll}
          style={{
            flex: 1,
            overflowY: 'auto',
            fontFamily: "'JetBrains Mono', ui-monospace, 'SF Mono', monospace",
            fontSize: '12px',
            lineHeight: 1.6,
          }}
        >
          {filtered.length === 0 ? (
            <div style={{ padding: '48px 16px' }}>
              <EmptyState
                icon={<span style={{ fontSize: 28, opacity: 0.3 }}>⏚</span>}
                title={entries.length === 0 ? '暂无日志' : '没有匹配的日志'}
                description={
                  entries.length === 0
                    ? '系统启动后将在这里显示日志'
                    : '试试调整筛选条件或搜索关键词'
                }
              />
            </div>
          ) : (
            filtered.map((entry, i) => {
              const cmp = entry.attrs?.cmp as string | undefined
              const cmpLabel = getCmpLabel(cmp)
              const meta = LEVEL_META[entry.level] || { color: 'var(--text)', label: entry.level, variant: 'neutral' }
              const key = `${entry.time}-${entry.level}-${entry.message}-${i}`
              const isExpanded = expandedKey === key
              const hasAttrs = entry.attrs && Object.keys(entry.attrs).length > 0 && !(Object.keys(entry.attrs).length === 1 && entry.attrs?.cmp)

              return (
                <div key={key}>
                  <div
                    onClick={() => setExpandedKey(isExpanded ? null : key)}
                    style={{
                      display: 'flex',
                      borderBottom: '1px solid var(--border)',
                      background: entry.level === 'ERROR'
                        ? 'rgba(239,68,68,0.04)'
                        : (isExpanded ? 'rgba(99,102,241,0.04)' : (i % 2 === 1 ? 'rgba(128,128,128,0.02)' : 'transparent')),
                      cursor: hasAttrs ? 'pointer' : 'default',
                      transition: 'background .1s',
                      animation: i < 20 ? `fadeIn .2s ease-out ${Math.min(i * 12, 180)}ms both` : undefined,
                    }}
                    onMouseEnter={e => (e.currentTarget.style.background = 'rgba(128,128,128,0.06)')}
                    onMouseLeave={e => {
                      e.currentTarget.style.background = entry.level === 'ERROR'
                        ? 'rgba(239,68,68,0.04)'
                        : (i % 2 === 1 ? 'rgba(128,128,128,0.02)' : 'transparent')
                    }}
                  >
                    <div style={{ width: 68, padding: '3px 10px', color: 'var(--text-muted)', whiteSpace: 'nowrap', flexShrink: 0, fontSize: '11px' }}>
                      {entry.time ? entry.time.slice(11, 19) : ''}
                    </div>
                    <div style={{ width: 48, padding: '3px 10px', flexShrink: 0, textAlign: 'center' }}>
                      <span style={{
                        display: 'inline-block',
                        padding: '0 5px',
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
                      width: 110,
                      padding: '3px 10px',
                      color: 'var(--text-muted)',
                      whiteSpace: 'nowrap',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      flexShrink: 0,
                      fontSize: '11px',
                    }} title={cmpLabel}>
                      {cmpLabel}
                    </div>
                    <div style={{
                      flex: 1,
                      padding: '3px 10px',
                      color: 'var(--text)',
                      wordBreak: 'break-all',
                      display: 'flex',
                      alignItems: 'center',
                      gap: 6,
                      minWidth: 0,
                    }}>
                      <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {entry.message}
                      </span>
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
                        <span style={{ color: '#ef4444', flexShrink: 0, overflow: 'hidden', textOverflow: 'ellipsis', maxWidth: 160, fontSize: '11px' }}>
                          {String(entry.attrs.error)}
                        </span>
                      )}
                    </div>
                    <div style={{
                      width: 36,
                      padding: '3px 10px',
                      flexShrink: 0,
                      textAlign: 'center',
                      color: hasAttrs ? 'var(--text-muted)' : 'transparent',
                      fontSize: '9px',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                    }}>
                      {hasAttrs ? (isExpanded ? '▲' : '▼') : '·'}
                    </div>
                  </div>

                  {/* Expanded attrs */}
                  {isExpanded && hasAttrs && (
                    <div style={{
                      background: 'rgba(99,102,241,0.03)',
                      borderBottom: '1px solid var(--border)',
                      padding: '8px 10px 8px 68px',
                      fontSize: '11px',
                      fontFamily: "'JetBrains Mono', ui-monospace, 'SF Mono', monospace",
                      lineHeight: 1.8,
                    }}>
                      {entry.attrs && Object.entries(entry.attrs).map(([k, v]) => {
                        if (k === 'cmp') return null
                        const val = typeof v === 'string' && v.length > 300 ? v.slice(0, 300) + '...' : String(v)
                        return (
                          <div key={k} style={{ display: 'flex', gap: 8 }}>
                            <span style={{ color: 'var(--text-muted)', minWidth: 80, flexShrink: 0 }}>{k}</span>
                            <span style={{
                              color: k === 'error' ? '#ef4444' : 'var(--text)',
                              wordBreak: 'break-all',
                            }}>
                              {val}
                            </span>
                          </div>
                        )
                      })}
                      {entry.source && (
                        <div style={{ display: 'flex', gap: 8, marginTop: 4, opacity: 0.6 }}>
                          <span style={{ minWidth: 80, flexShrink: 0 }}>source</span>
                          <span style={{ wordBreak: 'break-all' }}>{shortSource(entry.source)}</span>
                        </div>
                      )}
                    </div>
                  )}
                </div>
              )
            })
          )}
        </div>
      </div>

      <style>{`
        @keyframes fadeIn { from { opacity: 0; transform: translateY(3px); } to { opacity: 1; transform: translateY(0); } }
      `}</style>
    </div>
  )
}