import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createRefresher } from './refresh'

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

// Calls later() at t = 0, 25, 50, 75 and 100 ms.
function laterFiveTimes(r: { later(): void }) {
  r.later()
  for (let i = 0; i < 4; i++) {
    vi.advanceTimersByTime(25)
    r.later()
  }
}

describe('createRefresher later', () => {
  it('does not run before 200 ms have passed since the last later()', () => {
    const run = vi.fn()
    laterFiveTimes(createRefresher(run))
    vi.advanceTimersByTime(199)
    expect(run).toHaveBeenCalledTimes(0)
  })

  it('runs exactly once 200 ms after the last later()', () => {
    const run = vi.fn()
    laterFiveTimes(createRefresher(run))
    vi.advanceTimersByTime(200)
    expect(run).toHaveBeenCalledTimes(1)
    vi.advanceTimersByTime(1000)
    expect(run).toHaveBeenCalledTimes(1)
  })

  it('honours a custom delay', () => {
    const run = vi.fn()
    const r = createRefresher(run, 50)
    r.later()
    vi.advanceTimersByTime(49)
    expect(run).toHaveBeenCalledTimes(0)
    vi.advanceTimersByTime(1)
    expect(run).toHaveBeenCalledTimes(1)
  })
})

describe('createRefresher now', () => {
  it('runs immediately and drops the pending later()', () => {
    const run = vi.fn()
    const r = createRefresher(run)
    r.later()
    r.now()
    expect(run).toHaveBeenCalledTimes(1)
    vi.advanceTimersByTime(1000)
    expect(run).toHaveBeenCalledTimes(1)
  })
})

describe('createRefresher cancel', () => {
  it('drops the pending later() without running', () => {
    const run = vi.fn()
    const r = createRefresher(run)
    r.later()
    r.cancel()
    vi.advanceTimersByTime(1000)
    expect(run).toHaveBeenCalledTimes(0)
  })
})
