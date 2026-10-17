import type { app } from '../../wailsjs/go/models'

export interface TreemapDatum {
  name: string
  value: number
  id: number
  parentID: number
  isDir: boolean
  aggregate: boolean
  fileCount: number
  count: number
  skip: number
  children?: TreemapDatum[]
}

export function toTreemapData(view: app.View, aggregateLabel: (count: number) => string): TreemapDatum[] {
  return (view.Children ?? []).map((c): TreemapDatum => {
    if (c.Aggregate) {
      return {
        name: aggregateLabel(c.Count),
        value: c.Size,
        id: -1,
        parentID: view.ID,
        isDir: false,
        aggregate: true,
        fileCount: c.FileCount,
        count: c.Count,
        skip: c.Skip,
      }
    }
    const datum: TreemapDatum = {
      name: c.Name,
      value: c.Size,
      id: c.ID,
      parentID: view.ID,
      isDir: c.IsDir,
      aggregate: false,
      fileCount: c.FileCount,
      count: 0,
      skip: 0,
    }
    if (c.IsDir && c.Children && c.Children.length > 0) {
      datum.children = toTreemapData(c, aggregateLabel)
    }
    return datum
  })
}
