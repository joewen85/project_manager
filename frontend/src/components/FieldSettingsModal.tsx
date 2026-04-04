import { useEffect, useMemo, useState } from 'react'
import { Modal } from './Modal'

export interface FieldSettingItem {
  key: string
  label: string
  visible: boolean
  editable: boolean
  sortable: boolean
  searchable: boolean
  filterable: boolean
  custom: boolean
}

interface FieldSettingsModalProps<T extends FieldSettingItem> {
  open: boolean
  title: string
  fields: T[]
  defaultFields: T[]
  onClose: () => void
  onSave: (fields: T[]) => void
}

export function FieldSettingsModal<T extends FieldSettingItem>({
  open,
  title,
  fields,
  defaultFields,
  onClose,
  onSave
}: FieldSettingsModalProps<T>) {
  const [draftFields, setDraftFields] = useState<T[]>(fields)

  useEffect(() => {
    if (!open) return
    setDraftFields(fields)
  }, [open, fields])

  const hasChanges = useMemo(() => JSON.stringify(draftFields) !== JSON.stringify(fields), [draftFields, fields])

  const updateField = (key: string, patch: Partial<T>) => {
    setDraftFields((prev) => prev.map((field) => (field.key === key ? { ...field, ...patch } : field)))
  }

  const moveField = (index: number, direction: -1 | 1) => {
    setDraftFields((prev) => {
      const nextIndex = index + direction
      if (nextIndex < 0 || nextIndex >= prev.length) return prev
      const next = [...prev]
      const [target] = next.splice(index, 1)
      next.splice(nextIndex, 0, target)
      return next
    })
  }

  return (
    <Modal open={open} title={title} onClose={onClose}>
      <div className="field-settings-shell">
        <div className="field-settings-actions">
          <button type="button" className="btn secondary" onClick={() => setDraftFields(defaultFields)}>恢复默认</button>
        </div>

        <div className="field-settings-table-wrap">
          <table className="field-settings-table">
            <thead>
              <tr>
                <th>字段</th>
                <th>顺序</th>
                <th>显示</th>
                <th>可编辑</th>
                <th>可排序</th>
                <th>可搜索</th>
                <th>可筛选</th>
                <th>自定义字段</th>
              </tr>
            </thead>
            <tbody>
              {draftFields.map((field, index) => (
                <tr key={field.key}>
                  <td>{field.label}</td>
                  <td>
                    <div className="field-settings-order">
                      <button type="button" className="btn secondary" onClick={() => moveField(index, -1)} disabled={index === 0}>上移</button>
                      <button type="button" className="btn secondary" onClick={() => moveField(index, 1)} disabled={index === draftFields.length - 1}>下移</button>
                    </div>
                  </td>
                  <td><input type="checkbox" checked={field.visible} onChange={(event) => updateField(field.key, { visible: event.target.checked } as Partial<T>)} /></td>
                  <td><input type="checkbox" checked={field.editable} onChange={(event) => updateField(field.key, { editable: event.target.checked } as Partial<T>)} /></td>
                  <td><input type="checkbox" checked={field.sortable} onChange={(event) => updateField(field.key, { sortable: event.target.checked } as Partial<T>)} /></td>
                  <td><input type="checkbox" checked={field.searchable} onChange={(event) => updateField(field.key, { searchable: event.target.checked } as Partial<T>)} /></td>
                  <td><input type="checkbox" checked={field.filterable} onChange={(event) => updateField(field.key, { filterable: event.target.checked } as Partial<T>)} /></td>
                  <td><input type="checkbox" checked={field.custom} onChange={(event) => updateField(field.key, { custom: event.target.checked } as Partial<T>)} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <div className="row-actions">
          <button type="button" className="btn" onClick={() => onSave(draftFields)} disabled={!hasChanges}>保存设置</button>
          <button type="button" className="btn secondary" onClick={onClose}>取消</button>
        </div>
      </div>
    </Modal>
  )
}
