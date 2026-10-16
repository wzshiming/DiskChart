import type { app } from '../../wailsjs/go/models'

export interface TreemapDatum {
  name: string
  value: number
  id: number
  isDir: boolean
  aggregate: boolean
  fileCount: number
  count: number
  children?: TreemapDatum[]
}

export function toTreemapData(view: app.View, aggregateLabel: (count: number) => string): TreemapDatum[] {
  return (view.Children ?? []).map((c): TreemapDatum => {
    if (c.Aggregate) {
      return {
        name: aggregateLabel(c.Count),
        value: c.Size,
        id: -1,
        isDir: false,
        aggregate: true,
        fileCount: c.FileCount,
        count: c.Count,
      }
    }
    const datum: TreemapDatum = {
      name: c.Name,
      value: c.Size,
      id: c.ID,
      isDir: c.IsDir,
      aggregate: false,
      fileCount: c.FileCount,
      count: 0,
    }
    if (c.IsDir && c.Children && c.Children.length > 0) {
      datum.children = toTreemapData(c, aggregateLabel)
    }
    return datum
  })
}
