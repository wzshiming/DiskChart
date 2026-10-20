<script lang="ts">
  import { untrack } from 'svelte'
  import type { app } from '../../wailsjs/go/models'
  import { api } from '../lib/api'
  import type { TreemapDatum } from '../lib/chart'
  import { t } from '../lib/i18n'
  import type { Item, UiState } from '../lib/state'
  import AggregateDialog from './AggregateDialog.svelte'
  import DeleteDialog from './DeleteDialog.svelte'
  import DetailPanel from './DetailPanel.svelte'
  import SidePanel from './SidePanel.svelte'
  import UsageChart from './UsageChart.svelte'
  import Button from './ui/Button.svelte'
  import Icon from './ui/Icon.svelte'
  import Segmented from './ui/Segmented.svelte'
  import { baseName, type ChartKind } from './item'

  interface Props {
    ui: UiState
    dark: boolean
    onNavigate: (id: number) => void
    onSelect: (item: Item | null) => void
    onStale: () => void
    onRescan: () => void
  }
  let { ui, dark, onNavigate, onSelect, onStale, onRescan }: Props = $props()

  let kind = $state<ChartKind>('treemap')
  let view = $state.raw<app.View | null>(null)
  let viewError = $state('')
  let topFiles = $state.raw<app.Entry[] | null>(null)
  let topDirs = $state.raw<app.Entry[] | null>(null)
  let extensions = $state.raw<app.ExtReport | null>(null)
  let errors = $state.raw<app.ScanError[] | null>(null)
  let deleting = $state.raw<Item | null>(null)
  let aggregate = $state.raw<{ parentID: number; skip: number; count: number; size: number } | null>(null)
  let pathOf = $state.raw({ id: -2, path: '' })

  let listError = $state('')
  const loadError = $derived(listError || (view ? viewError : ''))

  // ui is replaced on every update, so effects key on primitives to refetch only when these change.
  const viewKey = $derived(`${ui.currentID}:${ui.revision}`)
  const listKey = $derived(`${ui.rootID}:${ui.revision}`)

  let viewSeq = 0
  $effect(() => {
    void viewKey
    aggregate = null
    const id = untrack(() => ui.currentID)
    if (id < 0) return
    const n = ++viewSeq
    api
      .subtree(id, 2, 40, 0.004)
      .then((v) => {
        if (n !== viewSeq) return
        view = v
        viewError = ''
      })
      .catch((e) => {
        if (n === viewSeq) viewError = String(e)
      })
  })

  let listSeq = 0
  $effect(() => {
    void listKey
    if (untrack(() => ui.rootID) < 0) return
    const n = ++listSeq
    Promise.all([api.topFiles(100), api.topDirs(50), api.extensions(12), api.scanErrors(500)])
      .then(([f, d, x, e]) => {
        if (n !== listSeq) return
        topFiles = f ?? []
        topDirs = d ?? []
        extensions = x
        errors = e ?? []
        listError = ''
      })
      .catch((err) => {
        if (n !== listSeq) return
        listError = String(err)
        topFiles = topFiles ?? []
        topDirs = topDirs ?? []
        errors = errors ?? []
      })
  })

  const detail = $derived.by((): Item | null => {
    if (ui.selected) return ui.selected
    if (!view || view.ID !== ui.currentID) return null
    return {
      id: view.ID,
      name: view.ID === ui.rootID ? baseName(view.Name) : view.Name,
      size: view.Size,
      apparent: view.Apparent,
      isDir: true,
      fileCount: view.FileCount,
      modTime: 0,
    }
  })

  // Only show a path fetched for the item currently in the detail panel.
  const path = $derived(pathOf.id === detail?.id ? pathOf.path : '')
  $effect(() => {
    const id = detail?.id
    if (id === undefined) return
    api
      .nodePath(id)
      .then((p) => {
        if (detail?.id === id) pathOf = { id, path: p }
      })
      .catch(() => {})
  })

  function selectFile(d: TreemapDatum) {
    const entry = topFiles?.find((e) => e.ID === d.id)
    onSelect({
      id: d.id,
      name: d.name,
      size: d.value,
      apparent: d.apparent,
      isDir: false,
      fileCount: 0,
      modTime: entry?.ModTime ?? 0,
    })
  }

  function pickFile(e: app.Entry) {
    const item: Item = {
      id: e.ID,
      name: e.Name,
      size: e.Size,
      apparent: e.Apparent,
      isDir: false,
      fileCount: 0,
      modTime: e.ModTime,
    }
    onSelect(item)
    api.crumbs(e.ID)
      .then((c) => {
        // A later pick or navigation wins over this stale lookup.
        if (ui.selected?.id !== item.id) return
        const parent = c?.at(-2)
        if (parent && parent.ID !== ui.currentID) {
          onNavigate(parent.ID)
          onSelect(item)
        }
      })
      .catch(() => {})
  }

  const kinds: ChartKind[] = ['treemap', 'sunburst']
</script>

<div class="flex h-full flex-col">
  {#if ui.stale}
    <div
      class="flex shrink-0 items-center gap-3 border-b border-amber-200 bg-amber-50 px-4 py-2 text-sm text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/40 dark:text-amber-200"
    >
      <Icon name="exclamation-triangle" class="size-4" />
      <span class="flex-1">{t('results.stale')}</span>
      <Button variant="warning" size="sm" icon="arrow-path" onclick={onRescan}>{t('results.rescan')}</Button>
    </div>
  {/if}

  {#if loadError}
    <div
      class="mx-3 mt-3 shrink-0 rounded-lg border border-red-200 bg-red-50 px-4 py-2 text-sm text-red-800 dark:border-red-900/60 dark:bg-red-950/40 dark:text-red-300"
      role="alert"
    >
      <span class="font-medium">{t('common.error')}</span>
      <span class="ml-2 break-all">{loadError}</span>
    </div>
  {/if}

  <div class="flex min-h-0 flex-1 gap-3 p-3">
    <section
      class="flex min-w-0 flex-[65_1_0%] flex-col overflow-hidden rounded-xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900"
    >
      <div class="flex shrink-0 items-center justify-end px-2 pt-2">
        <Segmented
          options={kinds.map((k) => ({ value: k, label: t('results.' + k) }))}
          value={kind}
          onChange={(v) => (kind = v as ChartKind)}
          ariaLabel={t('results.chartType')}
        />
      </div>
      <div class="relative min-h-0 flex-1 p-1">
        {#if view}
          <UsageChart
            {view}
            {kind}
            {dark}
            onOpenDir={onNavigate}
            onSelectFile={selectFile}
            onOpenAggregate={(d) =>
              (aggregate = { parentID: d.parentID, skip: d.skip, count: d.count, size: d.value })}
          />
          {#if view.Children.length === 0}
            <p class="absolute inset-0 flex items-center justify-center text-sm text-zinc-500">{t('results.empty')}</p>
          {/if}
        {:else if viewError}
          <p class="p-4 text-sm break-all text-red-600 dark:text-red-400">{t('common.error')}: {viewError}</p>
        {:else}
          <p class="absolute inset-0 flex items-center justify-center text-sm text-zinc-500">{t('common.loading')}</p>
        {/if}
      </div>
    </section>
    <aside class="flex min-w-0 flex-[35_1_0%] flex-col gap-3">
      <SidePanel
        {topFiles}
        {topDirs}
        {extensions}
        {errors}
        {dark}
        selectedID={ui.selected?.id ?? -2}
        onPickFile={pickFile}
        onPickDir={(e) => onNavigate(e.ID)}
      />
      <DetailPanel
        item={detail}
        {path}
        isRoot={detail?.id === ui.rootID}
        onDelete={() => (deleting = detail)}
      />
    </aside>
  </div>
</div>

<DeleteDialog
  item={deleting}
  {path}
  onClose={() => (deleting = null)}
  onDeleted={() => {
    deleting = null
    onSelect(null)
  }}
  {onStale}
/>

<AggregateDialog
  target={aggregate}
  selectedID={ui.selected?.id ?? -2}
  onClose={() => (aggregate = null)}
  onPick={(e) => {
    aggregate = null
    if (e.IsDir) onNavigate(e.ID)
    else pickFile(e)
  }}
/>
