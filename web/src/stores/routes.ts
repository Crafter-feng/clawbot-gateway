import { create } from 'zustand'
import { api } from '../api/client'

// 条件字段
export type ConditionField = 'message' | 'from_user' | 'to_user' | 'msg_type'

// 条件操作符
export type ConditionOperator = 'exact' | 'contains' | 'starts_with' | 'ends_with' | 'regex'

// 单个匹配条件
export interface RouteCondition {
  id: string
  field: ConditionField
  operator: ConditionOperator
  value: string
  case_sensitive: boolean
  negate: boolean
}

// 规则组
export interface RouteRuleGroup {
  id: string
  logic: 'and' | 'or'
  conditions: RouteCondition[]
}

// 路由规则
export interface RouteRule {
  id: number
  name: string
  backend_id: string
  priority: number
  enabled: boolean
  description: string
  groups: RouteRuleGroup[]
  group_logic: 'and' | 'or'
  created_at: string
  updated_at: string
}

// 匹配字段选项
export const CONDITION_FIELDS: { value: ConditionField; label: string }[] = [
  { value: 'message', label: '消息内容' },
  { value: 'from_user', label: '发送者' },
  { value: 'to_user', label: '接收者' },
  { value: 'msg_type', label: '消息类型' },
]

// 匹配操作符选项
export const CONDITION_OPERATORS: { value: ConditionOperator; label: string }[] = [
  { value: 'exact', label: '等于' },
  { value: 'contains', label: '包含' },
  { value: 'starts_with', label: '以...开头' },
  { value: 'ends_with', label: '以...结尾' },
  { value: 'regex', label: '正则匹配' },
]

interface RoutesState {
  items: RouteRule[]
  loading: boolean
  error: string | null
  fetch(): Promise<void>
  add(rule: Omit<RouteRule, 'id' | 'created_at' | 'updated_at'>): Promise<void>
  update(id: number, rule: Partial<RouteRule>): Promise<void>
  remove(id: number): Promise<void>
  toggleEnabled(id: number): Promise<void>
  reorder(ids: number[]): Promise<void>
  testMatch(message: string, userId: string): Promise<{ matched: boolean; backend_id: string; matched_by: string; rule_id: number }>
}

export const useRoutesStore = create<RoutesState>((set, get) => ({
  items: [],
  loading: false,
  error: null,

  async fetch() {
    set({ loading: true })
    try {
      const data = await api.get<{ rules: RouteRule[] }>('/api/v1/routes')
      set({ items: data.rules || [], loading: false })
    } catch (e) {
      const msg = e instanceof Error ? e.message : '获取路由规则失败'
      set({ items: [], loading: false, error: msg })
    }
  },

  async add(rule) {
    await api.post('/api/v1/routes', rule)
    await get().fetch()
  },

  async update(id, rule) {
    await api.put(`/api/v1/routes/${id}`, rule)
    await get().fetch()
  },

  async remove(id) {
    await api.del(`/api/v1/routes/${id}`)
    await get().fetch()
  },

  async toggleEnabled(id) {
    await api.put(`/api/v1/routes/${id}/toggle`)
    await get().fetch()
  },

  async reorder(ids) {
    await api.put('/api/v1/routes/reorder', { ids })
    await get().fetch()
  },

  async testMatch(message, userId) {
    return await api.post<{ matched: boolean; backend_id: string; matched_by: string; rule_id: number }>('/api/v1/routes/test', {
      message,
      user_id: userId,
      from_user: userId,
      msg_type: 'text',
    })
  },
}))
