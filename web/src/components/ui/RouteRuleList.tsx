import { useRoutesStore, type RouteRule, type RouteRuleGroup, type RouteCondition } from '../../stores/routes'
import Button from './Button'
import Tag from './Tag'

interface RouteRuleListProps {
  onEdit: (rule: RouteRule) => void
}

export default function RouteRuleList({ onEdit }: RouteRuleListProps) {
  const { items, remove, toggleEnabled } = useRoutesStore()

  const formatCondition = (condition: RouteCondition): string => {
    const fieldLabels: Record<string, string> = {
      message: '消息',
      from_user: '发送者',
      to_user: '接收者',
      msg_type: '类型',
    }
    const operatorLabels: Record<string, string> = {
      exact: '等于',
      contains: '包含',
      starts_with: '以...开头',
      ends_with: '以...结尾',
      regex: '匹配',
    }

    const field = fieldLabels[condition.field] || condition.field
    const operator = operatorLabels[condition.operator] || condition.operator
    const negate = condition.negate ? 'NOT ' : ''

    return `${negate}${field} ${operator} "${condition.value}"`
  }

  const formatGroup = (group: RouteRuleGroup): string => {
    if (group.conditions.length === 0) return '(空)'
    return group.conditions.map(formatCondition).join(` ${group.logic.toUpperCase()} `)
  }

  const formatRuleSummary = (rule: RouteRule): string => {
    if (rule.groups.length === 0) return '(无条件)'
    return rule.groups.map(formatGroup).join(` ${rule.group_logic.toUpperCase()} `)
  }

  if (items.length === 0) {
    return (
      <div className="empty-state">
        <div className="empty-state-title">暂无路由规则</div>
        <div className="empty-state-description">添加路由规则以自动分发消息</div>
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-2)' }}>
      {items.map((rule) => (
        <div key={rule.id} className="list-item">
          <div className="list-item-content" style={{ flex: 1, minWidth: 0 }}>
            <div className="list-item-info">
              <div className="list-item-title" style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
                <span style={{ color: 'var(--text-muted)', fontSize: 'var(--text-xs)' }}>P{rule.priority}</span>
                {rule.name}
                {!rule.enabled && <Tag variant="neutral">停用</Tag>}
              </div>
              <div className="list-item-subtitle" style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--text-xs)' }}>
                {formatRuleSummary(rule)} → {rule.backend_id}
              </div>
            </div>
          </div>
          <div className="list-item-actions">
            <Button variant="ghost" size="sm" onClick={() => toggleEnabled(rule.id)}>
              {rule.enabled ? '停用' : '启用'}
            </Button>
            <Button variant="ghost" size="sm" onClick={() => onEdit(rule)}>
              编辑
            </Button>
            <Button variant="ghost-danger" size="sm" onClick={() => remove(rule.id)}>
              删除
            </Button>
          </div>
        </div>
      ))}
    </div>
  )
}
