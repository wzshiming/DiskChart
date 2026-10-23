import { describe, it, expect } from 'vitest'
import { validatePath, validateRecommendedPath, validateName, appendUnique, rulesEqual } from './rules'

const platforms = ['darwin', 'linux', 'windows'] as const
const unix = ['darwin', 'linux'] as const

function on(list: readonly string[], paths: string[]): [string, string][] {
  return list.flatMap((platform) => paths.map((p): [string, string] => [p, platform]))
}

describe('validatePath', () => {
  it.each(on(platforms, ['', '   ']))('accepts blank %j on %s', (p, platform) => {
    expect(validatePath(p, platform)).toBe('')
  })

  it.each(on(platforms, ['~', '~/x', '  ~/x  ']))('accepts home-relative %j on %s', (p, platform) => {
    expect(validatePath(p, platform)).toBe('')
  })

  it.each(on(unix, ['/usr/local']))('accepts absolute POSIX path %j on %s', (p, platform) => {
    expect(validatePath(p, platform)).toBe('')
  })

  it('rejects an absolute POSIX path on windows', () => {
    expect(validatePath('/usr/local', 'windows')).toBe('rules.invalidPath')
  })

  it.each(on(platforms, ['relative/x', 'x', '~x', '~user/x']))('rejects relative %j on %s', (p, platform) => {
    expect(validatePath(p, platform)).toBe('rules.invalidPath')
  })

  it.each(['C:\\Users\\u', 'c:/tmp', '~\\x', '\\\\srv\\share\\x', '//srv/share'])('accepts %j on windows', (p) => {
    expect(validatePath(p, 'windows')).toBe('')
  })

  it.each(['C:', 'C:foo', '\\foo', '/x'])('rejects %j on windows', (p) => {
    expect(validatePath(p, 'windows')).toBe('rules.invalidPath')
  })

  it.each(on(unix, ['C:\\x', '~\\x']))('rejects windows-only form %j on %s', (p, platform) => {
    expect(validatePath(p, platform)).toBe('rules.invalidPath')
  })
})

describe('validateRecommendedPath', () => {
  it.each(['/usr/local/x', '~/x', '~/.cache'])('accepts %j on darwin', (p) => {
    expect(validateRecommendedPath(p, 'darwin')).toBe('')
  })

  it('reports an invalid path before breadth on darwin', () => {
    expect(validateRecommendedPath('rel', 'darwin')).toBe('rules.invalidPath')
  })

  it.each(['~', '~/', '~//', '/', '//'])('rejects %j as too broad on darwin', (p) => {
    expect(validateRecommendedPath(p, 'darwin')).toBe('rules.tooBroad')
  })

  it.each(['C:\\x', '~\\x', '\\\\srv\\share\\x'])('accepts %j on windows', (p) => {
    expect(validateRecommendedPath(p, 'windows')).toBe('')
  })

  it.each(['C:\\', 'c:/', 'C:\\\\', '\\\\srv\\share', '\\\\srv\\share\\', '~\\', '~'])(
    'rejects %j as too broad on windows',
    (p) => {
      expect(validateRecommendedPath(p, 'windows')).toBe('rules.tooBroad')
    },
  )

  it('reports an invalid path before breadth on windows', () => {
    expect(validateRecommendedPath('C:', 'windows')).toBe('rules.invalidPath')
  })
})

describe('validateName', () => {
  it.each(['', '  '])('accepts blank %j', (n) => {
    expect(validateName(n)).toBe('')
  })

  it.each(['node_modules', '.cache', 'a b'])('accepts %j', (n) => {
    expect(validateName(n)).toBe('')
  })

  it.each(['.', '..', 'a/b', 'a\\b', ' / '])('rejects %j', (n) => {
    expect(validateName(n)).toBe('rules.invalidName')
  })
})

describe('appendUnique', () => {
  it('appends a new item to a fresh array and leaves the input untouched', () => {
    const list = ['a']
    const next = appendUnique(list, 'b')
    expect(next).toEqual(['a', 'b'])
    expect(next).not.toBe(list)
    expect(list).toEqual(['a'])
  })

  it('returns the same array when the trimmed item is already present', () => {
    const list = ['a']
    expect(appendUnique(list, ' a ')).toBe(list)
  })

  it.each(['', '   '])('returns the same array for blank item %j', (item) => {
    const list = ['a']
    expect(appendUnique(list, item)).toBe(list)
  })

  it('trims the item before appending', () => {
    expect(appendUnique([], ' x ')).toEqual(['x'])
  })
})

describe('rulesEqual', () => {
  const rules = { Protected: ['/a', '/b'], Recommended: ['~/.cache'], Names: ['node_modules'] }

  it('is true for identical lists', () => {
    expect(rulesEqual(rules, structuredClone(rules))).toBe(true)
  })

  it('is false when a list holds the same items in another order', () => {
    expect(rulesEqual(rules, { ...rules, Protected: ['/b', '/a'] })).toBe(false)
  })

  it.each([
    ['missing', { Protected: ['/a'] }],
    ['null', { Protected: ['/a'], Recommended: null, Names: null }],
    ['undefined', { Protected: ['/a'], Recommended: undefined, Names: undefined }],
  ])('treats %s lists as empty', (_label, a) => {
    const b = { Protected: ['/a'], Recommended: [], Names: [] }
    expect(rulesEqual(a, b)).toBe(true)
    expect(rulesEqual(b, a)).toBe(true)
  })

  it('is false when an element differs', () => {
    expect(rulesEqual(rules, { ...rules, Names: ['.cache'] })).toBe(false)
  })

  it('is false when one list has an extra item', () => {
    expect(rulesEqual(rules, { ...rules, Recommended: ['~/.cache', '~/tmp'] })).toBe(false)
  })
})
