import { FormEvent, useEffect, useState } from 'react'
import { api, fetchPage, hasPermission, readApiError } from '../services/api'
import { DataState } from '../components/DataState'
import { FilterPanel } from '../components/FilterPanel'
import { Modal } from '../components/Modal'
import { Pagination } from '../components/Pagination'
import { SearchField } from '../components/SearchField'
import { Department, User } from '../types'
import { usePermissions } from '../hooks/usePermissions'

interface DepartmentForm {
  id?: number
  name: string
  description: string
  userIds: number[]
}

const initialForm: DepartmentForm = { name: '', description: '', userIds: [] }

export function DepartmentsPage() {
  const permissions = usePermissions()
  const canCreateDepartment = hasPermission('departments.create', permissions)
  const canUpdateDepartment = hasPermission('departments.update', permissions)
  const canDeleteDepartment = hasPermission('departments.delete', permissions)
  const [items, setItems] = useState<Department[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [form, setForm] = useState<DepartmentForm>(initialForm)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState('')
  const [formSuccess, setFormSuccess] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [total, setTotal] = useState(0)
  const activeFilterCount = Number(Boolean(keyword.trim()))

  const load = async () => {
    try {
      setLoading(true)
      setError('')
      const [departmentsPage, usersPage] = await Promise.all([
        fetchPage<Department>('/departments', { page, pageSize, keyword }, { page, pageSize }),
        fetchPage<User>('/users', { page: 1, pageSize: 100 }, { page: 1, pageSize: 100 }, { silent: true }).catch(() => ({ list: [] as User[], total: 0, page: 1, pageSize: 100 }))
      ])
      setItems(departmentsPage.list)
      setTotal(departmentsPage.total)
      setUsers(usersPage.list)
    } catch (loadError) {
      setError(readApiError(loadError, '部门列表加载失败'))
      setItems([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [page, pageSize, keyword])

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (form.id && !canUpdateDepartment) return
    if (!form.id && !canCreateDepartment) return
    try {
      setSubmitting(true)
      setFormError('')
      if (form.id) {
        await api.put(`/departments/${form.id}`, form)
      } else {
        await api.post('/departments', form)
      }
      setFormSuccess('保存成功')
      setModalOpen(false)
      setForm(initialForm)
      await load()
    } catch (submitError) {
      setFormError(readApiError(submitError, '保存部门失败'))
    } finally {
      setSubmitting(false)
    }
  }

  const edit = (item: Department) => {
    if (!canUpdateDepartment) return
    setForm({
      id: item.id,
      name: item.name,
      description: item.description,
      userIds: (item.users || []).map((user) => user.id)
    })
    setFormError('')
    setFormSuccess('')
    setModalOpen(true)
  }

  const openCreateModal = () => {
    if (!canCreateDepartment) return
    setForm(initialForm)
    setFormError('')
    setFormSuccess('')
    setModalOpen(true)
  }

  const onDelete = async (id: number) => {
    if (!canDeleteDepartment) return
    if (!confirm('确认删除该部门？')) return
    try {
      await api.delete(`/departments/${id}`)
      await load()
    } catch (deleteError) {
      setError(readApiError(deleteError, '删除部门失败'))
    }
  }

  return (
    <section className="page-section">
      <FilterPanel
        title="部门筛选"
        activeCount={activeFilterCount}
        actions={canCreateDepartment ? <button className="btn secondary" onClick={openCreateModal}>新增部门</button> : undefined}
        bodyClassName="form-grid"
      >
        <SearchField
          aria-label="部门关键词搜索"
          value={keywordInput}
          placeholder="名称/描述"
          onChange={setKeywordInput}
          onClear={() => {
            setPage(1)
            setKeyword('')
          }}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              setPage(1)
              setKeyword(keywordInput.trim())
            }
          }}
        />
        <div className="row-actions">
          <button className="btn" onClick={() => { setPage(1); setKeyword(keywordInput.trim()) }}>查询</button>
        </div>
      </FilterPanel>

      <div className="card">
        <DataState loading={loading} error={error} empty={!loading && !error && items.length === 0} emptyText="暂无部门数据" onRetry={() => { void load() }} />
        {!loading && !error && items.length > 0 && (
          <table className="responsive-table"><thead><tr><th>ID</th><th>名称</th><th>描述</th><th>成员数</th><th>操作</th></tr></thead><tbody>
            {items.map((item) => (
              <tr key={item.id}>
                <td data-label="ID">{item.id}</td><td data-label="名称">{item.name}</td><td data-label="描述">{item.description}</td><td data-label="成员数">{(item.users || []).length}</td>
                <td data-label="操作">
                  <div className="table-actions">
                    {canUpdateDepartment && <button className="btn secondary" onClick={() => edit(item)}>编辑</button>}
                    {canDeleteDepartment && <button className="btn danger" onClick={() => { void onDelete(item.id) }}>删除</button>}
                  </div>
                </td>
              </tr>
            ))}
          </tbody></table>
        )}
      </div>

      {!loading && !error && total > 0 && <Pagination total={total} page={page} pageSize={pageSize} onPageChange={setPage} onPageSizeChange={setPageSize} />}

      <Modal open={modalOpen} title={form.id ? '编辑部门' : '新增部门'} onClose={() => setModalOpen(false)}>
        <form className="form-grid" onSubmit={submit}>
          <label className="required-label" htmlFor="department-name">名称</label>
          <input id="department-name" value={form.name} onChange={(e) => setForm((prev) => ({ ...prev, name: e.target.value }))} required />
          <label htmlFor="department-description">描述</label>
          <input id="department-description" value={form.description} onChange={(e) => setForm((prev) => ({ ...prev, description: e.target.value }))} />
          <label htmlFor="department-users">成员</label>
          <select id="department-users" multiple value={form.userIds.map(String)} onChange={(event) => {
            const selected = Array.from(event.target.selectedOptions).map((option) => Number(option.value))
            setForm((prev) => ({ ...prev, userIds: selected }))
          }}>
            {users.map((user) => <option key={user.id} value={user.id}>{user.name}</option>)}
          </select>
          <div className="row-actions">
            <button type="submit" className="btn" disabled={submitting || (form.id ? !canUpdateDepartment : !canCreateDepartment)}>{submitting ? '保存中...' : '保存'}</button>
            <button type="button" className="btn secondary" onClick={() => setForm(initialForm)}>重置</button>
          </div>
          {formError && <p className="error">{formError}</p>}
          {formSuccess && <p className="success">{formSuccess}</p>}
        </form>
      </Modal>
    </section>
  )
}
