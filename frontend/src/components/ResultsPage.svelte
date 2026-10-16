<script lang="ts">
  import { untrack } from 'svelte'
  import type { app } from '../../wailsjs/go/models'
  import { api } from '../lib/api'
  import type { TreemapDatum } from '../lib/chart'
  import { t } from '../lib/i18n'
  import type { UiState } from '../lib/state'
  import Breadcrumbs from './Breadcrumbs.svelte'
  import DeleteDialog from './DeleteDialog.svelte'
  import DetailPanel from './DetailPanel.svelte'
  import SidePanel from './SidePanel.svelte'
  import UsageChart from './UsageChart.svelte'
  import Button from './ui/Button.svelte'
  import Icon from './ui/Icon.svelte'
  import { baseName, type ChartKind, type Item } from './item'

  interface Props {
    ui: UiState
    platform: string
    dark: boolean
    onNavigate: (id: number) => void
    onStale: () => void
    onRescan: () => void
  }
  let { ui, platform, dark, onNavigate, onStale, onRescan }: Props = $props()

  let kind = $state<ChartKind>('treemap')
  let view = $state.raw<app.View | null>(null)
  let crumbs = $state.raw<app.Crumb[]>([])
  let viewError = $state('')
  let topFiles = $state.raw<app.Entry[] | null>(null)
  let topDirs = $state.raw<app.Entry[] | null>(null)
  let extensions = $state.raw<app.ExtReport | null>(null)
  let errors = $state.raw<app.ScanError[] | null>(null)
  let selected = $state.raw<Item | null>(null)
  let deleting = $state.raw<Item | null>(null)
  let pathOf = $state.raw({ id: -2, path: '' })

  let listError = $state('')
  const loadError = $derived(listError || (view ? viewError : ''))

  // ui is replaced on every update, so effects key on primitives to refetch only when these change.
  const viewKey = $derived(`${ui.currentID}:${ui.revision}`)
  const listKey = $derived(`${ui.rootID}:${ui.revision}`)

  let viewSeq = 0
  $effect(() => {
    void viewKey
    const id = untrack(() => ui.currentID)
    if (id < 0) return
    const n = ++viewSeq
    Promise.all([api.subtree(id, 2, 40, 0.004), api.crumbs(id)])
      .then(([v, c]) => {
        if (n !== viewSeq) return
        view = v
        crumbs = c ?? []
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
    if (selected) return selected
    if (!view || view.ID !== ui.currentID) return null
    return {
      id: view.ID,
      name: view.ID === ui.rootID ? baseName(view.Name) : view.Name,
      size: view.Size,
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

  function openDir(id: number) {
    selected = null
    onNavigate(id)
  }

  function selectFile(d: TreemapDatum) {
    const entry = topFiles?.find((e) => e.ID === d.id)
    selected = { id: d.id, name: d.name, size: d.value, isDir: false, fileCount: 0, modTime: entry?.ModTime ?? 0 }
  }

  function pickFile(e: app.Entry) {
    selected = { id: e.ID, name: e.Name, size: e.Size, isDir: false, fileCount: 0, modTime: e.ModTime }
    api
      .crumbs(e.ID)
      .then((c) => {
        const parent = c?.at(-2)
        if (parent && parent.ID !== ui.currentID) onNavigate(parent.ID)
      })
      .catch(() => {})
  }

  const kinds: ChartKind[] = ['treemap', 'sunburst']
  const segBtn =
    'rounded-md px-3 py-1 text-xs font-medium transition-colors focus-visible:outline-2 focus-visible:outline-blue-500'
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

  <div class="flex shrink-0 items-center gap-3 px-3 pt-3">
    <div class="min-w-0 flex-1">
      <Breadcrumbs {crumbs} onNavigate={openDir} />
    </div>
    {#if ui.partial}
      <span
        class="shrink-0 rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800 dark:bg-amber-950 dark:text-amber-300"
      >
        {t('results.partial')}
      </span>
    {/if}
    <div class="flex shrink-0 rounded-lg bg-zinc-100 p-0.5 dark:bg-zinc-800" role="group">
      {#each kinds as k (k)}
        <button
          type="button"
          aria-pressed={kind === k}
          class="{segBtn} {kind === k
            ? 'bg-white text-zinc-900 shadow-sm dark:bg-zinc-700 dark:text-zinc-100'
            : 'text-zinc-500 hover:text-zinc-800 dark:text-zinc-400 dark:hover:text-zinc-200'}"
          onclick={() => (kind = k)}
        >
          {t('results.' + k)}
        </button>
      {/each}
    </div>
  </div>

  <div class="flex min-h-0 flex-1 gap-3 p-3">
    <section
      class="relative min-w-0 flex-[65_1_0%] overflow-hidden rounded-xl border border-zinc-200 bg-white p-1 dark:border-zinc-800 dark:bg-zinc-900"
    >
      {#if view}
        <UsageChart {view} {kind} {platform} {dark} onOpenDir={openDir} onSelectFile={selectFile} />
        {#if view.Children.length === 0}
          <p class="absolute inset-0 flex items-center justify-center text-sm text-zinc-500">{t('results.empty')}</p>
        {/if}
      {:else if viewError}
        <p class="p-4 text-sm break-all text-red-600 dark:text-red-400">{t('common.error')}: {viewError}</p>
      {:else}
        <p class="absolute inset-0 flex items-center justify-center text-sm text-zinc-500">{t('common.loading')}</p>
      {/if}
    </section>
    <aside class="flex min-w-0 flex-[35_1_0%] flex-col gap-3">
      <SidePanel
        {topFiles}
        {topDirs}
        {extensions}
        {errors}
        {platform}
        {dark}
        selectedID={selected?.id ?? -2}
        onPickFile={pickFile}
        onPickDir={(e) => openDir(e.ID)}
      />
      <DetailPanel
        item={detail}
        {path}
        {platform}
        isRoot={detail?.id === ui.rootID}
        onDelete={() => (deleting = detail)}
      />
    </aside>
  </div>
</div>

<DeleteDialog
  item={deleting}
  {path}
  {platform}
  onClose={() => (deleting = null)}
  onDeleted={() => {
    deleting = null
    selected = null
  }}
  {onStale}
/>
