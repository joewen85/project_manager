declare module 'frappe-gantt' {
  export interface GanttTask {
    id: string
    name: string
    start: string
    end: string
    progress?: number
    dependencies?: string
    custom_class?: string
    description?: string
  }

  export interface GanttViewMode {
    name: string
  }

  export interface GanttOptions {
    view_mode?: string
    view_modes?: string[]
    language?: string
    move_dependencies?: boolean
    readonly?: boolean
    readonly_progress?: boolean
    readonly_dates?: boolean
    popup?: false | ((context: unknown) => void)
    on_click?: (task: GanttTask) => void
    on_date_change?: (task: GanttTask, start: Date, end: Date) => void
    on_progress_change?: (task: GanttTask, progress: number) => void
    on_view_change?: (mode: GanttViewMode) => void
  }

  export default class Gantt {
    constructor(wrapper: HTMLElement | string | SVGElement, tasks: GanttTask[], options?: GanttOptions)
    refresh(tasks: GanttTask[]): void
    change_view_mode(mode?: string): void
    update_options(options: GanttOptions): void
  }
}
