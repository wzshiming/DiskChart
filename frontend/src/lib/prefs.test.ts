import { describe, it, expect, vi, afterEach } from 'vitest'

function fakeStorage(initial: Record<string, string> = {}) {
  const map = new Map<string, string>(Object.entries(initial))
  return {
    map,
    getItem: (k: string) => (map.has(k) ? map.get(k)! : null),
    setItem: (k: string, v: string) => {
      map.set(k, String(v))
    },
    removeItem: (k: string) => {
      map.delete(k)
    },
  }
}

function throwingStorage() {
  return {
    getItem: (): string | null => {
      throw new Error('storage denied')
    },
    setItem: (): void => {
      throw new Error('storage denied')
    },
    removeItem: (): void => {
      throw new Error('storage denied')
    },
  }
}

async function load(storage: unknown, language = 'en-US') {
  vi.resetModules()
  vi.stubGlobal('localStorage', storage)
  vi.stubGlobal('navigator', { language })
  return await import('./prefs.svelte')
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('defaultUnit', () => {
  it.each([
    ['darwin', 'si'],
    ['windows', 'iec'],
    ['linux', 'iec'],
    ['', 'iec'],
  ])('defaultUnit(%j) is %s', async (platform, want) => {
    const prefs = await load(undefined)
    expect(prefs.defaultUnit(platform)).toBe(want)
  })
})

describe('detectLocale', () => {
  it.each([
    ['zh', 'zh'],
    ['zh-CN', 'zh'],
    ['ZH-tw', 'zh'],
    ['zh-Hans-CN', 'zh'],
    ['en', 'en'],
    ['en-US', 'en'],
    ['ja', 'en'],
    ['', 'en'],
    [undefined, 'en'],
    [null, 'en'],
  ] as const)('detectLocale(%j) is %s', async (lang, want) => {
    const prefs = await load(undefined)
    expect(prefs.detectLocale(lang)).toBe(want)
  })
})

describe('unit preference', () => {
  it('getUnit follows the platform default when nothing is chosen', async () => {
    const prefs = await load(undefined)
    expect(prefs.getUnit()).toBe('iec')
    prefs.setPlatform('darwin')
    expect(prefs.getUnit()).toBe('si')
    prefs.setPlatform('linux')
    expect(prefs.getUnit()).toBe('iec')
  })

  it('setUnit persists the choice and overrides the platform default', async () => {
    const store = fakeStorage()
    const prefs = await load(store)
    prefs.setUnit('si')
    expect(store.map.get(prefs.UNIT_KEY)).toBe('si')
    expect(prefs.UNIT_KEY).toBe('DiskChart.unit')
    prefs.setPlatform('linux')
    expect(prefs.getUnit()).toBe('si')
  })

  it('stored iec wins over the darwin default', async () => {
    const prefs = await load(fakeStorage({ 'DiskChart.unit': 'iec' }))
    prefs.setPlatform('darwin')
    expect(prefs.getUnit()).toBe('iec')
  })

  it('stored si is used before any platform is set', async () => {
    const prefs = await load(fakeStorage({ 'DiskChart.unit': 'si' }))
    expect(prefs.getUnit()).toBe('si')
  })

  it.each(['KB', '', 'SI'])('stored unit %j is ignored as no choice', async (bad) => {
    const prefs = await load(fakeStorage({ 'DiskChart.unit': bad }))
    prefs.setPlatform('darwin')
    expect(prefs.getUnit()).toBe('si')
    prefs.setPlatform('linux')
    expect(prefs.getUnit()).toBe('iec')
  })
})

describe('locale preference', () => {
  it('stored zh wins over an en navigator', async () => {
    const prefs = await load(fakeStorage({ 'DiskChart.locale': 'zh' }), 'en-US')
    expect(prefs.getLocale()).toBe('zh')
  })

  it('invalid stored locale falls back to navigator detection', async () => {
    const prefs = await load(fakeStorage({ 'DiskChart.locale': 'fr' }), 'zh-CN')
    expect(prefs.getLocale()).toBe('zh')
  })

  it('without storage the locale is detected from navigator', async () => {
    const prefs = await load(undefined, 'zh-CN')
    expect(prefs.getLocale()).toBe('zh')
  })

  it('setLocale persists the choice and updates getLocale', async () => {
    const store = fakeStorage()
    const prefs = await load(store, 'zh-CN')
    prefs.setLocale('en')
    expect(prefs.LOCALE_KEY).toBe('DiskChart.locale')
    expect(store.map.get(prefs.LOCALE_KEY)).toBe('en')
    expect(prefs.getLocale()).toBe('en')
  })
})

describe('storage unavailable', () => {
  it('setters work without localStorage', async () => {
    const prefs = await load(undefined, 'en-US')
    expect(() => prefs.setUnit('si')).not.toThrow()
    expect(() => prefs.setLocale('zh')).not.toThrow()
    expect(prefs.getUnit()).toBe('si')
    expect(prefs.getLocale()).toBe('zh')
  })

  it('a throwing localStorage is ignored on load and on write', async () => {
    const prefs = await load(throwingStorage(), 'en-US')
    expect(() => prefs.setUnit('si')).not.toThrow()
    expect(() => prefs.setLocale('zh')).not.toThrow()
    expect(prefs.getUnit()).toBe('si')
    expect(prefs.getLocale()).toBe('zh')
  })
})

describe('i18n integration', () => {
  it('t follows the locale set through prefs', async () => {
    vi.resetModules()
    vi.stubGlobal('localStorage', fakeStorage())
    vi.stubGlobal('navigator', { language: 'en-US' })
    const i18n = await import('./i18n')
    const prefs = await import('./prefs.svelte')
    expect(i18n.getLocale).toBe(prefs.getLocale)
    prefs.setLocale('zh')
    expect(i18n.t('start.volumes')).toBe('磁盘')
    prefs.setLocale('en')
    expect(i18n.t('start.volumes')).toBe('Disks')
  })
})
