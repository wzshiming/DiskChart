import { describe, it, expect, beforeEach } from 'vitest'
import { dictionaries, setLocale, getLocale, t } from './i18n'

const requiredKeys = [
  'app.title', 'app.subtitle',
  'start.volumes', 'start.pickFolder', 'start.pickFolderTitle', 'start.sameDevice', 'start.sameDeviceHint',
  'start.systemDisk', 'start.used', 'start.free', 'start.of',
  'scanning.title', 'scanning.files', 'scanning.dirs', 'scanning.bytes', 'scanning.errors', 'scanning.elapsed',
  'scanning.cancel',
  'results.partial', 'results.stale', 'results.rescan', 'results.treemap', 'results.sunburst',
  'results.tabs.files', 'results.tabs.dirs', 'results.tabs.types', 'results.tabs.errors',
  'results.dominanceHint', 'results.aggregate', 'results.empty', 'results.chartType', 'results.location',
  'aggregate.hint', 'aggregate.loaded', 'aggregate.loadMore',
  'detail.size', 'detail.sizeOnDisk', 'detail.apparentSize', 'detail.files', 'detail.modified', 'detail.path',
  'detail.reveal', 'detail.copy', 'detail.copied', 'detail.delete',
  'delete.title', 'delete.body', 'delete.confirm', 'delete.cancel', 'delete.failed',
  'common.back', 'common.close', 'common.loading', 'common.error',
  'prefs.unit', 'prefs.unitSi', 'prefs.unitIec', 'prefs.language',
  'types.images', 'types.video', 'types.audio', 'types.archives', 'types.code', 'types.documents', 'types.apps',
  'types.other', 'types.noExt',
]

const locales = ['zh', 'en'] as const

beforeEach(() => setLocale('en'))

describe('dictionaries', () => {
  it('zh and en have identical key sets', () => {
    expect(Object.keys(dictionaries.zh).sort()).toEqual(Object.keys(dictionaries.en).sort())
  })

  it.each(locales)('%s contains every required key', (l) => {
    const missing = requiredKeys.filter((k) => !(k in dictionaries[l]))
    expect(missing).toEqual([])
  })

  it.each(locales)('%s values are all non-empty strings', (l) => {
    const bad = Object.entries(dictionaries[l])
      .filter(([, v]) => typeof v !== 'string' || v.length === 0)
      .map(([k]) => k)
    expect(bad).toEqual([])
  })

  it.each(locales)('%s results.aggregate contains {count}', (l) => {
    expect(dictionaries[l]['results.aggregate']).toContain('{count}')
  })

  it.each(locales)('%s aggregate.loaded contains {loaded} and {total}', (l) => {
    const s = dictionaries[l]['aggregate.loaded']
    expect(s).toContain('{loaded}')
    expect(s).toContain('{total}')
  })

  it.each(locales)('%s delete.body contains {path}, {size} and {files}', (l) => {
    const body = dictionaries[l]['delete.body']
    expect(body).toContain('{path}')
    expect(body).toContain('{size}')
    expect(body).toContain('{files}')
  })
})

describe('setLocale/getLocale', () => {
  it('getLocale returns the locale passed to setLocale', () => {
    setLocale('zh')
    expect(getLocale()).toBe('zh')
    setLocale('en')
    expect(getLocale()).toBe('en')
  })
})

describe('t', () => {
  it('t returns the key itself for a missing key', () => {
    expect(t('no.such.key')).toBe('no.such.key')
  })

  it('t uses the current locale dictionary', () => {
    setLocale('zh')
    expect(t('app.title')).toBe(dictionaries.zh['app.title'])
  })

  it('t interpolates {count} with the given param', () => {
    expect(t('results.aggregate', { count: 3 })).toBe(dictionaries.en['results.aggregate'].replace('{count}', '3'))
  })

  it('t keeps a placeholder whose param is not provided', () => {
    expect(t('results.aggregate')).toContain('{count}')
  })
})
