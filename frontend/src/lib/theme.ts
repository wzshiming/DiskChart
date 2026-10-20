const QUERY = '(prefers-color-scheme: dark)'

export function prefersDark(): boolean {
  return globalThis.matchMedia?.(QUERY).matches ?? false
}

export function onDarkChange(cb: (dark: boolean) => void): () => void {
  const mq = globalThis.matchMedia?.(QUERY)
  if (!mq) return () => {}
  const handler = (e: MediaQueryListEvent) => cb(e.matches)
  mq.addEventListener('change', handler)
  return () => mq.removeEventListener('change', handler)
}

export interface ChartTheme {
  palette: string[]
  aggregate: string
  border: string
  label: string
  upperLabel: string
  tooltipBg: string
  tooltipBorder: string
  tooltipText: string
  protected: string
  recommended: string
}

export function chartTheme(dark: boolean): ChartTheme {
  return dark
    ? {
        palette: ['#2563eb', '#059669', '#d97706', '#e11d48', '#7c3aed', '#0891b2', '#ea580c', '#65a30d', '#c026d3', '#0d9488'],
        aggregate: '#52525b',
        border: '#18181b',
        label: '#fafafa',
        upperLabel: '#e4e4e7',
        tooltipBg: '#27272a',
        tooltipBorder: '#3f3f46',
        tooltipText: '#f4f4f5',
        protected: '#f87171',
        recommended: '#34d399',
      }
    : {
        palette: ['#3b82f6', '#10b981', '#f59e0b', '#f43f5e', '#8b5cf6', '#06b6d4', '#f97316', '#84cc16', '#d946ef', '#14b8a6'],
        aggregate: '#a1a1aa',
        border: '#ffffff',
        label: '#ffffff',
        upperLabel: '#ffffff',
        tooltipBg: '#ffffff',
        tooltipBorder: '#e4e4e7',
        tooltipText: '#18181b',
        protected: '#dc2626',
        recommended: '#059669',
      }
}

export function mixColor(hex: string, target: string, t: number): string {
  const parse = (h: string) => [1, 3, 5].map((i) => parseInt(h.slice(i, i + 2), 16))
  const a = parse(hex)
  const b = parse(target)
  return '#' + a.map((v, i) => Math.round(v + (b[i] - v) * t).toString(16).padStart(2, '0')).join('')
}

export const categoryColors: Record<string, string> = {
  images: '#f59e0b',
  video: '#f43f5e',
  audio: '#8b5cf6',
  archives: '#f97316',
  code: '#10b981',
  documents: '#3b82f6',
  apps: '#06b6d4',
  other: '#a1a1aa',
}
