export interface ItemResult {
  id: number
  name: string
  ok: boolean
  error: string
  removedSize: number
}

export interface Selection {
  count: number
  size: number
}

export interface BatchSummary {
  ok: number
  failed: number
  freed: number
}

// Counts only IDs that are both selected and still listed in entries.
export function selectionSummary(
  entries: readonly { ID: number; Size: number }[],
  selected: ReadonlySet<number>,
): Selection {
  let count = 0
  let size = 0
  for (const e of entries) {
    if (!selected.has(e.ID)) continue
    count++
    size += e.Size
  }
  return { count, size }
}

export function batchSummary(results: readonly ItemResult[]): BatchSummary {
  let ok = 0
  let failed = 0
  let freed = 0
  for (const r of results) {
    if (r.ok) {
      ok++
      freed += r.removedSize
    } else {
      failed++
    }
  }
  return { ok, failed, freed }
}

export function toggleID(selected: ReadonlySet<number>, id: number): Set<number> {
  const next = new Set(selected)
  if (!next.delete(id)) next.add(id)
  return next
}
