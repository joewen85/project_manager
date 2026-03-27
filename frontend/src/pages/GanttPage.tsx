import { useMemo } from 'react'
import { useSearchParams } from 'react-router-dom'
import { GanttModule } from '../modules/gantt/GanttModule'

export function GanttPage() {
  const [searchParams] = useSearchParams()
  const initialProjectId = useMemo(() => {
    const raw = Number(searchParams.get('projectId') || 0)
    return Number.isFinite(raw) && raw > 0 ? raw : undefined
  }, [searchParams])

  return <GanttModule initialProjectId={initialProjectId} />
}
