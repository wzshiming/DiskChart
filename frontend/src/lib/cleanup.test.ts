import { describe, it, expect } from 'vitest'
import { selectionSummary, batchSummary, toggleID, type ItemResult } from './cleanup'

const entries = [
  { ID: 1, Size: 100 },
  { ID: 2, Size: 250 },
  { ID: 3, Size: 4000 },
]

describe('selectionSummary', () => {
  it('returns zero count and size for no entries and no selection', () => {
    expect(selectionSummary([], new Set<number>())).toEqual({ count: 0, size: 0 })
  })

  it('counts and sums only the selected entries', () => {
    expect(selectionSummary(entries, new Set([1, 3]))).toEqual({ count: 2, size: 4100 })
  })

  it('ignores selected ids that are not among the entries', () => {
    expect(selectionSummary(entries, new Set([2, 99]))).toEqual({ count: 1, size: 250 })
  })
})

describe('batchSummary', () => {
  it('returns all zeros for no results', () => {
    expect(batchSummary([])).toEqual({ ok: 0, failed: 0, freed: 0 })
  })

  it('counts ok and failed items and sums freed from ok items only', () => {
    const results: ItemResult[] = [
      { id: 1, name: 'a', ok: true, error: '', removedSize: 10 },
      { id: 2, name: 'b', ok: true, error: '', removedSize: 5 },
      { id: 3, name: 'c', ok: false, error: 'permission denied', removedSize: 7 },
    ]
    expect(batchSummary(results)).toEqual({ ok: 2, failed: 1, freed: 15 })
  })
})

describe('toggleID', () => {
  it('adds an id that is not selected', () => {
    expect(toggleID(new Set([1]), 2)).toEqual(new Set([1, 2]))
  })

  it('removes an id that is already selected', () => {
    expect(toggleID(new Set([1, 2]), 2)).toEqual(new Set([1]))
  })

  it('leaves the given set unchanged', () => {
    const selected = new Set([1, 2])
    toggleID(selected, 3)
    toggleID(selected, 1)
    expect(selected).toEqual(new Set([1, 2]))
  })

  it('returns a new set rather than the given one', () => {
    const selected = new Set([1])
    expect(toggleID(selected, 2)).not.toBe(selected)
    expect(toggleID(selected, 1)).not.toBe(selected)
  })
})
