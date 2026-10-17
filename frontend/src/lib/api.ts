import * as App from '../../wailsjs/go/app/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import type { app } from '../../wailsjs/go/models'

export const api = {
  info: App.Info,
  listVolumes: App.ListVolumes,
  pickDirectory: App.PickDirectory,
  startScan: App.StartScan,
  cancelScan: App.CancelScan,
  state: App.State,
  subtree: App.Subtree,
  children: App.Children,
  crumbs: App.Crumbs,
  topFiles: App.TopFiles,
  topDirs: App.TopDirs,
  extensions: App.Extensions,
  scanErrors: App.ScanErrors,
  nodePath: App.NodePath,
  reveal: App.Reveal,
  copyPath: App.CopyPath,
  delete: App.Delete,
}

export function onScanProgress(cb: (p: app.Progress) => void): () => void {
  return EventsOn('scan:progress', cb)
}

export function onScanFinished(cb: (e: app.FinishedEvent) => void): () => void {
  return EventsOn('scan:finished', cb)
}

export function onTreeChanged(cb: (e: app.TreeChangedEvent) => void): () => void {
  return EventsOn('tree:changed', cb)
}
