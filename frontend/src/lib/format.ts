import type { Locale } from './i18n'

export type Unit = 'si' | 'iec'

const LABELS: Record<Unit, string[]> = {
  si: ['B', 'KB', 'MB', 'GB', 'TB', 'PB'],
  iec: ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB'],
}

function intlLocale(locale: Locale): string {
  return locale === 'zh' ? 'zh-CN' : 'en-US'
}

export function formatBytes(bytes: number, unit: Unit): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const base = unit === 'si' ? 1000 : 1024
  const labels = LABELS[unit]
  if (bytes < base) return `${Math.round(bytes)} B`
  let k = 0
  while (k < labels.length - 1 && bytes / Math.pow(base, k + 1) >= 1) k++
  const value = bytes / Math.pow(base, k)
  const digits = value >= 100 ? 0 : value >= 10 ? 1 : 2
  return `${value.toFixed(digits)} ${labels[k]}`
}

export function formatPercent(part: number, whole: number): string {
  if (whole <= 0) return '0%'
  const p = (part / whole) * 100
  if (p >= 10) return `${Math.round(p)}%`
  if (p >= 1) return `${p.toFixed(1)}%`
  if (p > 0) return '<1%'
  return '0%'
}

export function formatDuration(ms: number): string {
  if (ms < 0) ms = 0
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  const totalSeconds = Math.floor(ms / 1000)
  if (ms < 3600000) {
    const m = Math.floor(totalSeconds / 60)
    const s = totalSeconds % 60
    return `${m}m ${String(s).padStart(2, '0')}s`
  }
  const h = Math.floor(totalSeconds / 3600)
  const m = Math.floor((totalSeconds % 3600) / 60)
  return `${h}h ${String(m).padStart(2, '0')}m`
}

export function formatDate(unixSeconds: number, locale: Locale): string {
  if (unixSeconds <= 0) return ''
  return new Date(unixSeconds * 1000).toLocaleString(intlLocale(locale), {
    dateStyle: 'medium',
    timeStyle: 'short',
  })
}

export function formatCount(n: number, locale: Locale): string {
  return n.toLocaleString(intlLocale(locale))
}
