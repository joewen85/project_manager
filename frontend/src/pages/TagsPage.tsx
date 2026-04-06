import { FormEvent, useEffect, useState } from 'react'
import { api, fetchPage, hasPermission, readApiError } from '../services/api'
import { DataState } from '../components/DataState'
import { FilterPanel } from '../components/FilterPanel'
import { Modal } from '../components/Modal'
import { Pagination } from '../components/Pagination'
import { SearchField } from '../components/SearchField'
import { Tag } from '../types'
import { usePermissions } from '../hooks/usePermissions'

interface TagForm {
  id?: number
  name: string
}

const initialForm: TagForm = { name: '' }

export function TagsPage() {
  const permissions = usePermissions()
  const canCreateTag = hasPermission('tags.create', permissions)
  const canUpdateTag = hasPermission('tags.update', permissions)
  const canDeleteTag = hasPermission('tags.delete', permissions)
  const [items, setItems] = useState<Tag[]>([])
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [form, setForm] = useState<TagForm>(initialForm)
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
      const tagsPage = await fetchPage<Tag>('/tags', { page, pageSize, keyword }, { page, pageSize })
      setItems(tagsPage.list)
      setTotal(tagsPage.total)
    } catch (loadError) {
      setError(readApiError(loadError, '标签列表加载失败'))
      setItems([])
      setTotal(0)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [page, pageSize, keyword])

  const openCreateModal = () => {
    if (!canCreateTag) return
    setForm(initialForm)
    setFormError('')
    setFormSuccess('')
    setModalOpen(true)
  }

  const edit = (item: Tag) => {
    if (!canUpdateTag) return
    setForm({ id: item.id, name: item.name })
    setFormError('')
    setFormSuccess('')
    setModalOpen(true)
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (form.id && !canUpdateTag) return
    if (!form.id && !canCreateTag) return
    try {
      setSubmitting(true)
      setFormError('')
      if (form.id) await api.put(`/tags/${form.id}`, form)
      else await api.post('/tags', form)
      setFormSuccess('保存成功')
      setModalOpen(false)
      setForm(initialForm)
      await load()
    } catch (submitError) {
      setFormError(readApiError(submitError, '保存标签失败'))
    } finally {
      setSubmitting(false)
    }
  }

  const onDelete = async (id: number) => {
    if (!canDeleteTag) return
    if (!confirm('确认删除该标签？')) return
    try {
      await api.delete(`/tags/${id}`)
      await load()
    } catch (deleteError) {
      setError(readApiError(deleteError, '删除标签失败'))
    }
  }

  return (
    <section className="page-section">
      <FilterPanel
        title="标签筛选"
        activeCount={activeFilterCount}
        actions={canCreateTag ? <button className="btn secondary" onClick={openCreateModal}>新增标签</button> : undefined}
        bodyClassName="form-grid"
      >
        <SearchField
          aria-label="标签关键词搜索"
          value={keywordInput}
          placeholder="标签名称"
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
        <DataState loading={loading} error={error} empty={!loading && !error && items.length === 0} emptyText="暂无标签数据" onRetry={() => { void load() }} />
        {!loading && !error && items.length > 0 && (
          <table className="responsive-table"><thead><tr><th>ID</th><th>标签名称</th><th>关联任务数</th><th>操作</th></tr></thead><tbody>
            {items.map((item) => (
              <tr key={item.id}>
                <td data-label="ID">{item.id}</td>
                <td data-label="标签名称">{item.name}</td>
                <td data-label="关联任务数">{Number(item.taskCount || 0)}</td>
                <td data-label="操作">
                  <div className="table-actions">
                    {canUpdateTag && <button className="btn secondary" onClick={() => edit(item)}>编辑</button>}
                    {canDeleteTag && <button className="btn danger" onClick={() => { void onDelete(item.id) }}>删除</button>}
                  </div>
                </td>
              </tr>
            ))}
          </tbody></table>
        )}
      </div>

      {!loading && !error && total > 0 && <Pagination total={total} page={page} pageSize={pageSize} onPageChange={setPage} onPageSizeChange={setPageSize} />}

      <Modal open={modalOpen} title={form.id ? '编辑标签' : '新增标签'} onClose={() => setModalOpen(false)}>
        <form className="form-grid" onSubmit={submit}>
          <label className="required-label" htmlFor="tag-name">标签名称</label>
          <input id="tag-name" value={form.name} onChange={(e) => setForm((prev) => ({ ...prev, name: e.target.value }))} required />
          <div className="row-actions">
            <button type="submit" className="btn" disabled={submitting || (form.id ? !canUpdateTag : !canCreateTag)}>{submitting ? '保存中...' : '保存'}</button>
            <button type="button" className="btn secondary" onClick={() => setForm(initialForm)}>重置</button>
          </div>
          {formError && <p className="error">{formError}</p>}
          {formSuccess && <p className="success">{formSuccess}</p>}
        </form>
      </Modal>
    </section>
  )
}
