import dayjs from 'dayjs'

export const formatDateTime = (value?: string | null) => {
  if (!value) return '-'
  const parsed = dayjs(value)
  if (!parsed.isValid()) return '-'
  return parsed.format('YYYY-MM-DD HH:mm')
}
