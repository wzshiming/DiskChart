import type { app } from '../../wailsjs/go/models'

export type Phase = 'start' | 'scanning' | 'results'

export interface Item {
  id: number
  name: string
  size: number
  apparent: number
  isDir: boolean
  fileCount: number
  modTime: number
}

export interface UiState {
  phase: Phase
  root: string
  rootID: number
  currentID: number
  selected: Item | null
  status: string
  progress: app.Progress | null
  summary: app.Summary | null
  error: string
  partial: boolean
  stale: boolean
  revision: number
}

export type UiEvent =
  | { type: 'started'; root: string }
  | { type: 'progress'; progress: app.Progress }
  | { type: 'finished'; event: app.FinishedEvent }
  | { type: 'treeChanged'; event: app.TreeChangedEvent }
  | { type: 'navigate'; id: number }
  | { type: 'select'; item: Item | null }
  | { type: 'stale' }
  | { type: 'reset' }

export const initialState: UiState = {
  phase: 'start',
  root: '',
  rootID: -1,
  currentID: -1,
  selected: null,
  status: 'idle',
  progress: null,
  summary: null,
  error: '',
  partial: false,
  stale: false,
  revision: 0,
}

export function reduce(s: UiState, e: UiEvent): UiState {
  switch (e.type) {
    case 'started':
      return {
        ...s,
        phase: 'scanning',
        root: e.root,
        rootID: -1,
        currentID: -1,
        selected: null,
        status: 'scanning',
        progress: null,
        summary: null,
        error: '',
        partial: false,
        stale: false,
        revision: s.revision + 1,
      }
    case 'progress':
      return { ...s, progress: e.progress }
    case 'finished': {
      const status = e.event.Status
      if (status === 'done' || status === 'cancelled') {
        return {
          ...s,
          phase: 'results',
          status,
          rootID: e.event.RootID,
          currentID: e.event.RootID,
          summary: e.event.Summary,
          partial: status === 'cancelled',
          error: '',
          revision: s.revision + 1,
        }
      }
      return {
        ...s,
        phase: 'start',
        status,
        error: e.event.Error || 'failed',
        rootID: -1,
        currentID: -1,
        summary: null,
        revision: s.revision + 1,
      }
    }
    case 'treeChanged':
      if (s.currentID === e.event.RemovedID) {
        return { ...s, currentID: e.event.ParentID, revision: s.revision + 1 }
      }
      return { ...s, revision: s.revision + 1 }
    case 'navigate':
      return { ...s, currentID: e.id, selected: null }
    case 'select':
      return { ...s, selected: e.item }
    case 'stale':
      return { ...s, stale: true }
    case 'reset':
      return { ...initialState }
  }
}
