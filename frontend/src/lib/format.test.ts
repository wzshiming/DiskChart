import { describe, it, expect } from 'vitest'
import { formatBytes, formatPercent, formatDuration, formatDate, formatCount } from './format'

describe('formatBytes si', () => {
  it.each([
    [0, '0 B'],
    [-5, '0 B'],
    [NaN, '0 B'],
    [512, '512 B'],
    [999, '999 B'],
    [1000, '1.00 KB'],
    [1500, '1.50 KB'],
    [15000, '15.0 KB'],
    [150000, '150 KB'],
    [1e9, '1.00 GB'],
    [1e15, '1.00 PB'],
    [1e18, '1000 PB'],
  ])('formatBytes(%s, si) is %s', (bytes, want) => {
    expect(formatBytes(bytes, 'si')).toBe(want)
  })
})

describe('formatBytes iec', () => {
  it.each([
    [1000, '1000 B'],
    [1023, '1023 B'],
    [1024, '1.00 KiB'],
    [1536, '1.50 KiB'],
    [1048576, '1.00 MiB'],
    [15728640, '15.0 MiB'],
    [1073741824, '1.00 GiB'],
    [2 ** 40, '1.00 TiB'],
    [2 ** 50, '1.00 PiB'],
    [2 ** 60, '1024 PiB'],
  ])('formatBytes(%s, iec) is %s', (bytes, want) => {
    expect(formatBytes(bytes, 'iec')).toBe(want)
  })
})

describe('formatPercent', () => {
  it.each([
    [10, 0, '0%'],
    [50, 100, '50%'],
    [1.23, 100, '1.2%'],
    [0.5, 100, '<1%'],
    [0, 100, '0%'],
    [99.6, 100, '100%'],
  ])('formatPercent(%s, %s) is %s', (part, whole, want) => {
    expect(formatPercent(part, whole)).toBe(want)
  })
})

describe('formatDuration', () => {
  it.each([
    [-1, '0.0s'],
    [1500, '1.5s'],
    [59999, '60.0s'],
    [125000, '2m 05s'],
    [3720000, '1h 02m'],
  ])('formatDuration(%s) is %s', (ms, want) => {
    expect(formatDuration(ms)).toBe(want)
  })
})

describe('formatDate', () => {
  it('formatDate returns empty string for zero seconds', () => {
    expect(formatDate(0, 'en')).toBe('')
  })

  it('formatDate returns empty string for negative seconds', () => {
    expect(formatDate(-1, 'zh')).toBe('')
  })

  it('formatDate returns a non-empty string for a positive timestamp', () => {
    const got = formatDate(1700000000, 'en')
    expect(typeof got).toBe('string')
    expect(got.length).toBeGreaterThan(0)
  })
})

describe('formatCount', () => {
  it('formatCount groups thousands for en', () => {
    expect(formatCount(1234567, 'en')).toBe('1,234,567')
  })
})
