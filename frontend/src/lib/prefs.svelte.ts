import type { Unit } from './format'
import type { Locale } from './i18n'

export const UNIT_KEY = 'DiskChart.unit'
export const LOCALE_KEY = 'DiskChart.locale'

export function detectLocale(language: string | undefined | null): Locale {
  return /^zh/i.test(language ?? '') ? 'zh' : 'en'
}

export function defaultUnit(platform: string): Unit {
  return platform === 'darwin' ? 'si' : 'iec'
}

function readStored(key: string): string | null {
  try {
    return globalThis.localStorage?.getItem(key) ?? null
  } catch {
    return null
  }
}

function writeStored(key: string, value: string): void {
  try {
    globalThis.localStorage?.setItem(key, value)
  } catch {
    // storage unavailable or full: keep the in-memory choice only
  }
}

function storedUnit(): Unit | null {
  const v = readStored(UNIT_KEY)
  return v === 'si' || v === 'iec' ? v : null
}

function storedLocale(): Locale | null {
  const v = readStored(LOCALE_KEY)
  return v === 'zh' || v === 'en' ? v : null
}

let unitChoice = $state<Unit | null>(storedUnit())
let platform = $state('')
let locale = $state<Locale>(storedLocale() ?? detectLocale(globalThis.navigator?.language))

export function getUnit(): Unit {
  return unitChoice ?? defaultUnit(platform)
}

export function setUnit(u: Unit): void {
  unitChoice = u
  writeStored(UNIT_KEY, u)
}

export function setPlatform(p: string): void {
  platform = p
}

export function getPlatform(): string {
  return platform
}

export function getLocale(): Locale {
  return locale
}

export function setLocale(l: Locale): void {
  locale = l
  writeStored(LOCALE_KEY, l)
}
