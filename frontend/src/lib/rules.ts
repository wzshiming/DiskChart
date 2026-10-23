export interface RuleLists {
  Protected?: string[] | null
  Recommended?: string[] | null
  Names?: string[] | null
}

// '' is valid: an empty input shows no error, it is simply not addable.
export function validatePath(p: string, platform: string): '' | 'rules.invalidPath' {
  const s = p.trim()
  if (s === '' || s === '~' || s.startsWith('~/')) return ''
  if (platform === 'windows') {
    if (s.startsWith('~\\') || /^[A-Za-z]:[\\/]/.test(s) || /^[\\/]{2}/.test(s)) return ''
  } else if (s.startsWith('/')) {
    return ''
  }
  return 'rules.invalidPath'
}

export function validateRecommendedPath(
  p: string,
  platform: string,
): '' | 'rules.invalidPath' | 'rules.tooBroad' {
  const invalid = validatePath(p, platform)
  if (invalid) return invalid
  const s = p.trim()
  const broad =
    platform === 'windows'
      ? /^~[\\/]+$/.test(s) || /^[A-Za-z]:[\\/]+$/.test(s) || /^[\\/]{2}[^\\/]+[\\/]+[^\\/]+[\\/]*$/.test(s)
      : /^~\/+$/.test(s) || /^\/+$/.test(s)
  return s === '~' || broad ? 'rules.tooBroad' : ''
}

export function validateName(n: string): '' | 'rules.invalidName' {
  const s = n.trim()
  if (s === '') return ''
  return s === '.' || s === '..' || /[\\/]/.test(s) ? 'rules.invalidName' : ''
}

export function appendUnique(list: string[], item: string): string[] {
  const s = item.trim()
  if (s === '' || list.includes(s)) return list
  return [...list, s]
}

export function rulesEqual(a: RuleLists, b: RuleLists): boolean {
  return (
    sameList(a.Protected, b.Protected) && sameList(a.Recommended, b.Recommended) && sameList(a.Names, b.Names)
  )
}

function sameList(x: string[] | null | undefined, y: string[] | null | undefined): boolean {
  const a = x ?? []
  const b = y ?? []
  return a.length === b.length && a.every((v, i) => v === b[i])
}
