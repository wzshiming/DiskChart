export type ChartKind = 'treemap' | 'sunburst'

export function baseName(p: string): string {
  const trimmed = p.replace(/[\\/]+$/, '')
  const last = trimmed.split(/[\\/]/).pop()
  return last || p
}
