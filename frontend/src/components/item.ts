export type ChartKind = 'treemap' | 'sunburst'

export interface Item {
  id: number
  name: string
  size: number
  isDir: boolean
  fileCount: number
  modTime: number
}

export function baseName(p: string): string {
  const trimmed = p.replace(/[\\/]+$/, '')
  const last = trimmed.split(/[\\/]/).pop()
  return last || p
}
