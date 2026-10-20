export type Advice = '' | 'protected' | 'recommended'

export type Reason =
  | 'root'
  | 'volume'
  | 'home'
  | 'system'
  | 'userdata'
  | 'contains'
  | 'cache'
  | 'temp'
  | 'trash'
  | 'logs'
  | 'rebuildable'
  | 'custom'

const reasons: ReadonlySet<string> = new Set<Reason>([
  'root',
  'volume',
  'home',
  'system',
  'userdata',
  'contains',
  'cache',
  'temp',
  'trash',
  'logs',
  'rebuildable',
  'custom',
])

export function asAdvice(v?: string): Advice {
  return v === 'protected' || v === 'recommended' ? v : ''
}

export function adviceKey(a?: string): string {
  const advice = asAdvice(a)
  return advice ? 'advice.' + advice : ''
}

export function reasonKey(r?: string): string {
  return r !== undefined && reasons.has(r) ? 'advice.reason.' + r : ''
}

// Compact one-line label, e.g. "Protected · System folder". Without advice it is '' regardless of
// the reason; without a known reason only the advice word remains.
export function adviceLabel(a: string | undefined, r: string | undefined, t: (key: string) => string): string {
  const advice = adviceKey(a)
  if (!advice) return ''
  const reason = reasonKey(r)
  return [t(advice), reason ? t(reason) : ''].filter(Boolean).join(' · ')
}
