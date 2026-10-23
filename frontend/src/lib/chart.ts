import type { app } from '../../wailsjs/go/models'
import { asAdvice, type Advice } from './advice'

export interface TreemapDatum {
  name: string
  value: number
  apparent: number
  id: number
  parentID: number
  isDir: boolean
  aggregate: boolean
  fileCount: number
  count: number
  skip: number
  advice: Advice
  reason: string
  children?: TreemapDatum[]
}

export function toTreemapData(view: app.View, aggregateLabel: (count: number) => string): TreemapDatum[] {
  return (view.Children ?? []).map((c): TreemapDatum => {
    if (c.Aggregate) {
      return {
        name: aggregateLabel(c.Count),
        value: c.Size,
        apparent: c.Apparent,
        id: -1,
        parentID: view.ID,
        isDir: false,
        aggregate: true,
        fileCount: c.FileCount,
        count: c.Count,
        skip: c.Skip,
        advice: '',
        reason: '',
      }
    }
    const datum: TreemapDatum = {
      name: c.Name,
      value: c.Size,
      apparent: c.Apparent,
      id: c.ID,
      parentID: view.ID,
      isDir: c.IsDir,
      aggregate: false,
      fileCount: c.FileCount,
      count: 0,
      skip: 0,
      advice: asAdvice(c.Advice),
      reason: c.Reason ?? '',
    }
    if (c.IsDir && c.Children && c.Children.length > 0) {
      datum.children = toTreemapData(c, aggregateLabel)
    }
    return datum
  })
}
