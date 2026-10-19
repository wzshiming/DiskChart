import { describe, it, expect } from 'vitest'
import { app } from '../../wailsjs/go/models'
import { toTreemapData } from './chart'

const aggregateLabel = (count: number) => `${count} small items`

// Every Apparent is 2 * Size + 1, so an apparent read from Size never matches.
const view = app.View.createFrom({
  ID: 10, Name: '/root', Size: 100, Apparent: 201, IsDir: true, FileCount: 3, Aggregate: false, Count: 0, Skip: 0,
  Children: [
    {
      ID: 1, Name: 'a', Size: 60, Apparent: 121, IsDir: true, FileCount: 2, Aggregate: false, Count: 0, Skip: 0,
      Children: [{ ID: 3, Name: 'f', Size: 60, Apparent: 121, IsDir: false, FileCount: 0, Aggregate: false, Count: 0, Skip: 0, Children: [] }],
    },
    { ID: 2, Name: 'z', Size: 0, Apparent: 1, IsDir: true, FileCount: 0, Aggregate: false, Count: 0, Skip: 0, Children: [] },
    { ID: -1, Name: '', Size: 40, Apparent: 81, IsDir: false, FileCount: 5, Aggregate: true, Count: 7, Skip: 2, Children: [] },
  ],
})

const nested = app.View.createFrom({
  ID: 20, Name: '/n', Size: 90, Apparent: 181, IsDir: true, FileCount: 9, Aggregate: false, Count: 0, Skip: 0,
  Children: [
    {
      ID: 21, Name: 'sub', Size: 90, Apparent: 181, IsDir: true, FileCount: 9, Aggregate: false, Count: 0, Skip: 0,
      Children: [
        { ID: 22, Name: 'big', Size: 50, Apparent: 101, IsDir: false, FileCount: 0, Aggregate: false, Count: 0, Skip: 0, Children: [] },
        { ID: -1, Name: '', Size: 40, Apparent: 81, IsDir: false, FileCount: 8, Aggregate: true, Count: 8, Skip: 1, Children: [] },
      ],
    },
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
    expect(empty).toEqual({
      name: 'z', value: 0, apparent: 1, id: 2, parentID: 10, isDir: true, aggregate: false, fileCount: 0, count: 0, skip: 0,
    })
    expect('children' in empty).toBe(false)
  })

  it('maps an aggregate item using aggregateLabel(Count)', () => {
    const agg = toTreemapData(view, aggregateLabel)[2]
    expect(agg).toEqual({
      name: aggregateLabel(7), value: 40, apparent: 81, id: -1, parentID: 10, isDir: false, aggregate: true, fileCount: 5,
      count: 7, skip: 2,
    })
    expect('children' in agg).toBe(false)
  })

  it('sets parentID to the view ID and zero count/skip on ordinary items', () => {
    const data = toTreemapData(view, aggregateLabel).filter((d) => !d.aggregate)
    expect(data.map((d) => [d.parentID, d.skip, d.count])).toEqual([[10, 0, 0], [10, 0, 0]])
  })

  it('sets parentID of a grandchild to its directory ID', () => {
    const big = toTreemapData(nested, aggregateLabel)[0].children![0]
    expect(big).toMatchObject({ id: 22, parentID: 21, skip: 0, count: 0 })
  })

  it('maps a nested aggregate with the nested Skip and the directory as parent', () => {
    const agg = toTreemapData(nested, aggregateLabel)[0].children![1]
    expect(agg).toEqual({
      name: aggregateLabel(8), value: 40, apparent: 81, id: -1, parentID: 21, isDir: false, aggregate: true, fileCount: 8,
      count: 8, skip: 1,
    })
  })

  it('takes apparent of ordinary items from their View.Apparent', () => {
    const [dir] = toTreemapData(view, aggregateLabel)
    expect(dir.apparent).toBe(view.Children[0].Apparent)
    expect(dir.children![0].apparent).toBe(view.Children[0].Children[0].Apparent)
  })

  it('takes apparent of an aggregate item from its View.Apparent', () => {
    const agg = toTreemapData(view, aggregateLabel)[2]
    expect(agg.apparent).toBe(view.Children[2].Apparent)
  })

  it('returns [] when view Children is null', () => {
    const leaf = app.View.createFrom({ ID: 0, Name: '/r', Size: 0, IsDir: true, FileCount: 0, Aggregate: false, Count: 0, Children: null })
    expect(toTreemapData(leaf, aggregateLabel)).toEqual([])
  })
})
