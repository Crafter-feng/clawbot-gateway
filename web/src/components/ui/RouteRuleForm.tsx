import { useState, useEffect } from 'react'
import { useRoutesStore, type RouteRule, type RouteRuleGroup, type RouteCondition, type ConditionField, type ConditionOperator, CONDITION_FIELDS, CONDITION_OPERATORS } from '../../stores/routes'
import { useBackendsStore } from '../../stores/backends'
import Button from './Button'
import Input from './Input'
import Select from './Select'

interface RouteRuleFormProps {
  rule?: RouteRule | null
  onClose: () => void
  onSave: () => void
}

export default function RouteRuleForm({ rule, onClose, onSave }: RouteRuleFormProps) {
  const { add, update, testMatch } = useRoutesStore()
  const backends = useBackendsStore()

  const [name, setName] = useState(rule?.name || '')
  const [backendId, setBackendId] = useState(rule?.backend_id || '')
  const [priority, setPriority] = useState(rule?.priority || 1)
  const [enabled, setEnabled] = useState(rule?.enabled ?? true)
  const [description, setDescription] = useState(rule?.description || '')
  const [groups, setGroups] = useState<RouteRuleGroup[]>(rule?.groups || [])
  const [groupLogic, setGroupLogic] = useState<'and' | 'or'>(rule?.group_logic || 'and')
  const [saving, setSaving] = useState(false)

  // 测试匹配
  const [testMessage, setTestMessage] = useState('')
  const [testResult, setTestResult] = useState<{ matched: boolean; backend_id: string } | null>(null)

  useEffect(() => {
    backends.fetch()
  }, [])

  const handleTest = async () => {
    if (!testMessage.trim()) return
    // TODO: 使用当前登录用户 ID 而非硬编码的 'test_user'
    // 当前认证系统不提供用户 ID，后续可从 JWT 或 user store 获取
    const result = await testMatch(testMessage, 'test_user')
    setTestResult(result)
  }
  const handleAddGroup = () => {
    setGroups([
      ...groups,
      {
        id: `g_${Date.now()}`,
        logic: 'and',
        conditions: [],
      },
    ])
  }

  const handleRemoveGroup = (groupId: string) => {
    setGroups(groups.filter((g) => g.id !== groupId))
  }

  const handleGroupLogicChange = (groupId: string, logic: 'and' | 'or') => {
    setGroups(groups.map((g) => (g.id === groupId ? { ...g, logic } : g)))
  }

  const handleAddCondition = (groupId: string) => {
    setGroups(
      groups.map((g) =>
        g.id === groupId
          ? {
              ...g,
              conditions: [
                ...g.conditions,
                {
                  id: `c_${Date.now()}`,
                  field: 'message' as ConditionField,
                  operator: 'contains' as ConditionOperator,
                  value: '',
                  case_sensitive: false,
                  negate: false,
                },
              ],
            }
          : g
      )
    )
  }

  const handleRemoveCondition = (groupId: string, conditionId: string) => {
    setGroups(
      groups.map((g) =>
        g.id === groupId
          ? { ...g, conditions: g.conditions.filter((c) => c.id !== conditionId) }
          : g
      )
    )
  }

  const handleConditionChange = (groupId: string, conditionId: string, field: keyof RouteCondition, value: string | boolean) => {
    setGroups(
      groups.map((g) =>
        g.id === groupId
          ? {
              ...g,
              conditions: g.conditions.map((c) =>
                c.id === conditionId ? { ...c, [field]: value } : c
              ),
            }
          : g
      )
    )
  }

  const handleSave = async () => {
    // 过滤掉值为空的条件
    const filteredGroups: RouteRuleGroup[] = groups.map((g) => ({
      ...g,
      conditions: g.conditions.filter((c) => c.value.trim() !== ''),
    })).filter((g) => g.conditions.length > 0)

    if (!name.trim() || !backendId || filteredGroups.length === 0) return

    try {
      const ruleData = {
        name: name.trim(),
        backend_id: backendId,
        priority,
        enabled,
        description: description.trim(),
        groups: filteredGroups,
        group_logic: groupLogic,
      }

      if (rule) {
        await update(rule.id, ruleData)
      } else {
        await add(ruleData)
      }
      onSave()
    } catch (e) {
      const msg = e instanceof Error ? e.message : '保存失败'
      // 错误处理 - 使用 toast 或 setError
      console.error('保存路由规则失败:', msg)
      alert('保存失败: ' + msg)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
      {/* 基本信息 */}
      <div className="form-grid form-grid-2">
        <Input
          label="规则名称"
          placeholder="例如：天气查询"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
        <Select
          label="目标后端"
          value={backendId}
          onChange={(e) => setBackendId(e.target.value)}
        >
          <option value="">选择后端</option>
          {backends.items.map((b) => (
            <option key={b.id} value={b.id}>{b.name}</option>
          ))}
        </Select>
      </div>

      <div className="form-grid form-grid-3">
        <Input
          label="优先级"
          type="number"
          value={priority}
          onChange={(e) => setPriority(parseInt(e.target.value) || 1)}
          hint="数字越小优先级越高"
        />
        <Input
          label="描述"
          placeholder="可选"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
        <div className="input-group">
          <label className="input-label">状态</label>
          <div className="checkbox-group" style={{ paddingTop: '8px' }}>
            <input
              type="checkbox"
              className="checkbox"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
            />
            <span className="checkbox-label">启用</span>
          </div>
        </div>
      </div>

      {/* 条件配置 */}
      <div className="card" style={{ background: 'var(--bg-primary)' }}>
        <div className="card-header">
          <h3 className="card-title" style={{ fontSize: 'var(--text-sm)' }}>条件配置</h3>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)', marginBottom: 'var(--space-4)' }}>
          <span style={{ fontSize: 'var(--text-sm)', color: 'var(--text-secondary)' }}>组间逻辑:</span>
          <Select
            value={groupLogic}
            onChange={(e) => setGroupLogic(e.target.value as 'and' | 'or')}
            style={{ width: 'auto', minWidth: '100px' }}
          >
            <option value="and">且 (AND)</option>
            <option value="or">或 (OR)</option>
          </Select>
        </div>

        {groups.map((group) => (
          <div key={group.id} className="card" style={{ background: 'var(--bg-secondary)', marginBottom: 'var(--space-3)' }}>
            <div className="card-header">
              <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
                <span style={{ fontSize: 'var(--text-sm)', fontWeight: 600 }}>组</span>
                <Select
                  value={group.logic}
                  onChange={(e) => handleGroupLogicChange(group.id, e.target.value as 'and' | 'or')}
                  style={{ width: 'auto', minWidth: '80px' }}
                >
                  <option value="and">且</option>
                  <option value="or">或</option>
                </Select>
              </div>
              <Button variant="ghost-danger" size="sm" onClick={() => handleRemoveGroup(group.id)}>
                删除组
              </Button>
            </div>

            {group.conditions.map((condition) => (
              <div key={condition.id} className="card" style={{ background: 'var(--bg-primary)', marginBottom: 'var(--space-2)' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)', flexWrap: 'wrap' }}>
                  <div className="checkbox-group">
                    <input
                      type="checkbox"
                      className="checkbox"
                      checked={condition.negate}
                      onChange={(e) => handleConditionChange(group.id, condition.id, 'negate', e.target.checked)}
                    />
                    <span className="checkbox-label">非</span>
                  </div>

                  <Select
                    value={condition.field}
                    onChange={(e) => handleConditionChange(group.id, condition.id, 'field', e.target.value)}
                    style={{ width: 'auto', minWidth: '100px' }}
                  >
                    {CONDITION_FIELDS.map((f) => (
                      <option key={f.value} value={f.value}>{f.label}</option>
                    ))}
                  </Select>

                  <Select
                    value={condition.operator}
                    onChange={(e) => handleConditionChange(group.id, condition.id, 'operator', e.target.value)}
                    style={{ width: 'auto', minWidth: '100px' }}
                  >
                    {CONDITION_OPERATORS.map((o) => (
                      <option key={o.value} value={o.value}>{o.label}</option>
                    ))}
                  </Select>

                  <Input
                    placeholder="匹配值"
                    value={condition.value}
                    onChange={(e) => handleConditionChange(group.id, condition.id, 'value', e.target.value)}
                    style={{ flex: 1, minWidth: '120px' }}
                  />

                  <div className="checkbox-group">
                    <input
                      type="checkbox"
                      className="checkbox"
                      checked={condition.case_sensitive}
                      onChange={(e) => handleConditionChange(group.id, condition.id, 'case_sensitive', e.target.checked)}
                    />
                    <span className="checkbox-label">区分大小写</span>
                  </div>

                  <Button variant="ghost-danger" size="sm" onClick={() => handleRemoveCondition(group.id, condition.id)}>
                    删除
                  </Button>
                </div>
              </div>
            ))}

            <Button variant="ghost" size="sm" onClick={() => handleAddCondition(group.id)}>
              + 添加条件
            </Button>
          </div>
        ))}

        <Button variant="ghost" size="sm" onClick={handleAddGroup}>
          + 添加规则组
        </Button>
      </div>

      {/* 匹配预览 */}
      <div className="card" style={{ background: 'var(--bg-primary)' }}>
        <div className="card-header">
          <h3 className="card-title" style={{ fontSize: 'var(--text-sm)' }}>匹配预览</h3>
        </div>
        <div style={{ display: 'flex', gap: 'var(--space-2)', alignItems: 'flex-end' }}>
          <Input
            placeholder="输入测试消息..."
            value={testMessage}
            onChange={(e) => setTestMessage(e.target.value)}
            style={{ flex: 1 }}
          />
          <Button variant="secondary" size="sm" onClick={handleTest}>
            测试
          </Button>
        </div>
        {testResult && (
          <div style={{
            marginTop: 'var(--space-3)',
            padding: 'var(--space-3)',
            borderRadius: 'var(--radius-md)',
            background: testResult.matched ? 'var(--success-dim)' : 'var(--danger-dim)',
            fontSize: 'var(--text-sm)',
          }}>
            {testResult.matched ? (
              <span style={{ color: 'var(--success)' }}>✅ 匹配 - 转发到 {testResult.backend_id}</span>
            ) : (
              <span style={{ color: 'var(--danger)' }}>❌ 不匹配</span>
            )}
          </div>
        )}
      </div>

      {/* 正则表达式安全提示 */}
      {groups.some((g) => g.conditions.some((c) => c.operator === 'regex')) && (
        <div style={{
          padding: 'var(--space-3)',
          borderRadius: 'var(--radius-md)',
          background: 'var(--warning-dim)',
          fontSize: 'var(--text-sm)',
          color: 'var(--warning)',
        }}>
          ⚠️ 正则表达式长度限制 200 字符，禁止嵌套量词
        </div>
      )}

      {/* 操作按钮 */}
      <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 'var(--space-2)' }}>
        <Button variant="ghost" onClick={onClose}>
          取消
        </Button>
        <Button onClick={handleSave} loading={saving} disabled={!name.trim() || !backendId || groups.length === 0}>
          保存
        </Button>
      </div>
    </div>
  )
}
