import { describe, it, expect } from 'vitest'
import { adviceKey, reasonKey, asAdvice, type Reason } from './advice'
import { dictionaries } from './i18n'

const reasons: Reason[] = [
  'root', 'volume', 'home', 'system', 'userdata', 'contains',
  'cache', 'temp', 'trash', 'logs', 'rebuildable', 'custom',
]

const blanks: [string, string | undefined][] = [
  ['an empty string', ''],
  ['an unknown value', 'bogus'],
  ['undefined', undefined],
]

const locales = ['zh', 'en'] as const

describe('adviceKey', () => {
  it('maps protected to advice.protected', () => {
    expect(adviceKey('protected')).toBe('advice.protected')
  })

  it('maps recommended to advice.recommended', () => {
    expect(adviceKey('recommended')).toBe('advice.recommended')
  })

  it.each(blanks)('returns "" for %s', (_label, v) => {
    expect(adviceKey(v)).toBe('')
  })
})

describe('reasonKey', () => {
  it.each(reasons)('maps %s to its advice.reason key', (r) => {
    expect(reasonKey(r)).toBe(`advice.reason.${r}`)
  })

  it.each([...blanks, ['a value outside the Reason set', 'nope']])('returns "" for %s', (_label, v) => {
    expect(reasonKey(v)).toBe('')
  })
})

describe('asAdvice', () => {
  it('keeps protected', () => {
    expect(asAdvice('protected')).toBe('protected')
  })

  it('keeps recommended', () => {
    expect(asAdvice('recommended')).toBe('recommended')
  })

  it.each([...blanks, ['a one-letter value', 'x']])('narrows %s to ""', (_label, v) => {
    expect(asAdvice(v)).toBe('')
  })
})

describe('dictionary coverage', () => {
  const keys = [adviceKey('protected'), adviceKey('recommended'), ...reasons.map((r) => reasonKey(r))]

  it('adviceKey and reasonKey produce 14 distinct keys', () => {
    expect(new Set(keys).size).toBe(14)
  })

  it.each(locales)('%s defines every adviceKey and reasonKey result', (l) => {
    const missing = keys.filter((k) => !(k in dictionaries[l]))
    expect(missing).toEqual([])
  })
})
