import { describe, it, expect } from 'vitest'
import { app } from '../../wailsjs/go/models'
import { toTreemapData } from './chart'

const aggregateLabel = (count: number) => `${count} small items`

const view = app.View.createFrom({
  ID: 0, Name: '/root', Size: 100, IsDir: true, FileCount: 3, Aggregate: false, Count: 0,
  Children: [
    {
      ID: 1, Name: 'a', Size: 60, IsDir: true, FileCount: 2, Aggregate: false, Count: 0,
      Children: [{ ID: 3, Name: 'f', Size: 60, IsDir: false, FileCount: 0, Aggregate: false, Count: 0, Children: [] }],
    },
    { ID: 2, Name: 'z', Size: 0, IsDir: true, FileCount: 0, Aggregate: false, Count: 0, Children: [] },
    { ID: -1, Name: '', Size: 40, IsDir: false, FileCount: 5, Aggregate: true, Count: 7, Children: [] },
  ],
})

describe('toTreemapData', () => {
  it('maps every child of the view in order', () => {
    const data = toTreemapData(view, aggregateLabel)
    expect(data).toHaveLength(3)
    expect(data.map((d) => d.id)).toEqual([1, 2, -1])
  })

  it('maps a non-empty directory with recursive children', () => {
    const [dir] = toTreemapData(view, aggregateLabel)
    expect(dir).toMatchObject({ name: 'a', value: 60, id: 1, isDir: true, aggregate: false, fileCount: 2, count: 0 })
    expect(dir.children).toHaveLength(1)
    expect(dir.children![0]).toMatchObject({ name: 'f', value: 60, id: 3, isDir: false, aggregate: false, fileCount: 0, count: 0 })
  })

  it('omits the children key on a file', () => {
    const file = toTreemapData(view, aggregateLabel)[0].children![0]
    expect('children' in file).toBe(false)
  })

  it('keeps a zero-size empty directory without a children key', () => {
    const empty = toTreemapData(view, aggregateLabel)[1]
    expect(empty).toEqual({ name: 'z', value: 0, id: 2, isDir: true, aggregate: false, fileCount: 0, count: 0 })
    expect('children' in empty).toBe(false)
  })

  it('maps an aggregate item using aggregateLabel(Count)', () => {
    const agg = toTreemapData(view, aggregateLabel)[2]
    expect(agg).toEqual({
      name: aggregateLabel(7), value: 40, id: -1, isDir: false, aggregate: true, fileCount: 5, count: 7,
    })
    expect('children' in agg).toBe(false)
  })

  it('returns [] when view Children is null', () => {
    const leaf = app.View.createFrom({ ID: 0, Name: '/r', Size: 0, IsDir: true, FileCount: 0, Aggregate: false, Count: 0, Children: null })
    expect(toTreemapData(leaf, aggregateLabel)).toEqual([])
  })
})
