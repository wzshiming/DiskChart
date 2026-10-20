import { describe, it, expect } from 'vitest'
import { app } from '../../wailsjs/go/models'
import { initialState, reduce, type Item, type UiState, type UiEvent } from './state'

type FinishedPayload = Extract<UiEvent, { type: 'finished' }>['event']
type TreeChangedPayload = Extract<UiEvent, { type: 'treeChanged' }>['event']

const summary = app.Summary.createFrom({ Size: 100, Files: 3, Dirs: 2, Errors: 0, DurationMs: 1500 })
const progress = app.Progress.createFrom({ Dirs: 1, Files: 2, Bytes: 30, Errors: 0, CurrentPath: '/r/a', ElapsedMs: 10 })

function finished(Status: string, Error = '', RootID = 0): UiEvent {
  const event = { Status, Error, RootID, Summary: summary } as FinishedPayload
  return { type: 'finished', event }
}

function treeChanged(RemovedID: number, ParentID: number): UiEvent {
  const event = { RemovedID, ParentID } as TreeChangedPayload
  return { type: 'treeChanged', event }
}

const results: UiState = {
  ...initialState,
  phase: 'results',
  root: '/r',
  rootID: 0,
  currentID: 5,
  status: 'done',
  summary,
  revision: 4,
}

const scanning: UiState = { ...initialState, phase: 'scanning', root: '/r', status: 'scanning', revision: 2 }

const item: Item = { id: 7, name: 'f.txt', size: 10, apparent: 21, isDir: false, fileCount: 0, modTime: 1, advice: '', reason: '' }

function run(s: UiState, e: UiEvent): UiState {
  const before = structuredClone(s)
  const next = reduce(s, e)
  expect(s, 'reduce mutated its input state').toEqual(before)
  expect(next).not.toBe(s)
  return next
}

describe('initialState', () => {
  it('initialState has the documented field values', () => {
    expect(initialState).toEqual({
      phase: 'start',
      root: '',
      rootID: -1,
      currentID: -1,
      status: 'idle',
      progress: null,
      summary: null,
      error: '',
      partial: false,
      stale: false,
      revision: 0,
      selected: null,
    })
  })
})

describe('reduce', () => {
  it('started resets scan fields, enters scanning and bumps revision', () => {
    const prev: UiState = { ...results, error: 'x', partial: true, stale: true, progress, selected: item }
    const next = run(prev, { type: 'started', root: '/new' })
    expect(next).toEqual({
      phase: 'scanning',
      root: '/new',
      rootID: -1,
      currentID: -1,
      status: 'scanning',
      progress: null,
      summary: null,
      error: '',
      partial: false,
      stale: false,
      revision: prev.revision + 1,
      selected: null,
    })
  })

  it('progress updates only progress and keeps revision', () => {
    const next = run(scanning, { type: 'progress', progress })
    expect(next).toEqual({ ...scanning, progress })
  })

  it('finished done enters results with root and summary', () => {
    const next = run(scanning, finished('done', '', 7))
    expect(next.phase).toBe('results')
    expect(next.status).toBe('done')
    expect(next.rootID).toBe(7)
    expect(next.currentID).toBe(7)
    expect(next.summary).toEqual(summary)
    expect(next.partial).toBe(false)
    expect(next.error).toBe('')
    expect(next.revision).toBe(scanning.revision + 1)
  })

  it('finished cancelled enters results marked partial', () => {
    const next = run(scanning, finished('cancelled', '', 7))
    expect(next.phase).toBe('results')
    expect(next.status).toBe('cancelled')
    expect(next.rootID).toBe(7)
    expect(next.currentID).toBe(7)
    expect(next.summary).toEqual(summary)
    expect(next.partial).toBe(true)
    expect(next.revision).toBe(scanning.revision + 1)
  })

  it('finished failed returns to start with the event Error', () => {
    const next = run(scanning, finished('failed', 'boom', 7))
    expect(next.phase).toBe('start')
    expect(next.status).toBe('failed')
    expect(next.error).toBe('boom')
    expect(next.rootID).toBe(-1)
    expect(next.currentID).toBe(-1)
    expect(next.summary).toBeNull()
    expect(next.revision).toBe(scanning.revision + 1)
  })

  it('finished failed with empty Error reports "failed"', () => {
    const next = run(scanning, finished('failed', '', 7))
    expect(next.error).toBe('failed')
  })

  it('treeChanged removing the current node moves currentID to the parent', () => {
    const next = run(results, treeChanged(5, 2))
    expect(next.currentID).toBe(2)
    expect(next.revision).toBe(results.revision + 1)
  })

  it('treeChanged removing another node keeps currentID but bumps revision', () => {
    const next = run(results, treeChanged(9, 2))
    expect(next.currentID).toBe(5)
    expect(next.revision).toBe(results.revision + 1)
  })

  it('navigate sets currentID and keeps revision', () => {
    const next = run(results, { type: 'navigate', id: 11 })
    expect(next).toEqual({ ...results, currentID: 11 })
  })

  it('navigate clears the selected item', () => {
    const next = run({ ...results, selected: item }, { type: 'navigate', id: 11 })
    expect(next).toEqual({ ...results, currentID: 11, selected: null })
  })

  it('select sets the selected item', () => {
    const next = run(results, { type: 'select', item })
    expect(next).toEqual({ ...results, selected: item })
  })

  it('select keeps the advice and reason of the item', () => {
    const protectedItem: Item = { ...item, advice: 'protected', reason: 'root' }
    const next = run(results, { type: 'select', item: protectedItem })
    expect(next.selected).toEqual(protectedItem)
  })

  it('select with null clears the selected item', () => {
    const next = run({ ...results, selected: item }, { type: 'select', item: null })
    expect(next).toEqual({ ...results, selected: null })
  })

  it('stale marks the state stale', () => {
    const next = run(results, { type: 'stale' })
    expect(next).toEqual({ ...results, stale: true })
  })

  it('stale keeps the selected item', () => {
    const next = run({ ...results, selected: item }, { type: 'stale' })
    expect(next.selected).toBe(item)
  })

  it('reset returns a fresh copy of initialState', () => {
    const next = run(results, { type: 'reset' })
    expect(next).toEqual(initialState)
    expect(next).not.toBe(initialState)
  })
})
