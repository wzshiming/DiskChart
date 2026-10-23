<script lang="ts">
  import type { app } from '../../wailsjs/go/models'
  import { adviceLabel } from '../lib/advice'
  import { api } from '../lib/api'
  import { batchSummary, selectionSummary, toggleID, type BatchSummary, type ItemResult } from '../lib/cleanup'
  import { formatBytes, formatCount } from '../lib/format'
  import { getLocale, t } from '../lib/i18n'
  import { getUnit } from '../lib/prefs.svelte'
  import Button from './ui/Button.svelte'
  import Icon from './ui/Icon.svelte'

  interface Props {
    report: app.CleanupReport | null
    onClose: () => void
    onFinished: (s: BatchSummary) => void
    onStale: () => void
  }
  let { report, onClose, onFinished, onStale }: Props = $props()

  type Phase = 'idle' | 'running' | 'done'

  let dlg = $state<HTMLDialogElement>()
  let phase = $state<Phase>('idle')
  let selected = $state.raw<ReadonlySet<number>>(new Set())
  let results = $state.raw<ItemResult[]>([])
  let total = $state(0)
  let current = $state('')
  let cancelRequested = $state(false)
  let cancelled = $state(false)
  // onFinished/onClose have fired for this batch.
  let finished = false

  const entries = $derived(report?.Entries ?? [])
  const listedSize = $derived(entries.reduce((sum, e) => sum + e.Size, 0))
  const selection = $derived(selectionSummary(entries, selected))
  const allSelected = $derived(entries.length > 0 && selection.count === entries.length)
  const summary = $derived(batchSummary(results))
  const failures = $derived(results.filter((r) => !r.ok))
  const running = $derived(phase === 'running')

  function reset(r: app.CleanupReport | null) {
    phase = 'idle'
    selected = new Set(r?.Entries.map((e) => e.ID))
    results = []
    total = 0
    current = ''
    cancelRequested = false
    cancelled = false
    finished = false
  }

  $effect(() => {
    if (report && dlg && !dlg.open) {
      reset(report)
      dlg.showModal()
    } else if (!report) {
      reset(null)
      if (dlg?.open) dlg.close()
    }
  })

  function toggleAll() {
    selected = allSelected ? new Set<number>() : new Set(entries.map((e) => e.ID))
  }

  async function start() {
    const r = report
    if (!r || phase !== 'idle') return
    const queue = r.Entries.filter((e) => selected.has(e.ID))
    if (queue.length === 0) return
    phase = 'running'
    total = queue.length
    const done: ItemResult[] = []
    for (const e of queue) {
      current = e.Name
      const res: ItemResult = { id: e.ID, name: e.Name, ok: true, error: '', removedSize: 0 }
      try {
        res.removedSize = (await api.delete(e.ID)).RemovedSize
      } catch (err) {
        res.ok = false
        res.error = String(err)
      }
      // The dialog was reset underneath us; stop without reporting.
      if (report !== r) return
      done.push(res)
      results = [...done]
      if (cancelRequested) break
    }
    cancelled = done.length < queue.length
    if (batchSummary(done).failed > 0) {
      const s = await api.state().catch(() => null)
      if (report !== r) return
      if (s?.TreeStale) onStale()
    }
    phase = 'done'
  }

  function cancel() {
    if (running) cancelRequested = true
    else dlg?.close()
  }

  function closed() {
    if (running) {
      // A forced close mid-batch (e.g. a second Esc) counts as Cancel; keep the progress visible.
      cancelRequested = true
      try {
        dlg?.showModal()
      } catch {
        requestAnimationFrame(() => dlg?.showModal())
      }
      return
    }
    if (finished) return
    finished = true
    if (phase !== 'idle') onFinished(batchSummary(results))
    onClose()
  }
</script>

<dialog
  bind:this={dlg}
  aria-labelledby="cleanup-dialog-title"
  class="m-auto w-[min(560px,92vw)] rounded-2xl border border-zinc-200 bg-white p-6 text-zinc-900 shadow-2xl backdrop:bg-black/40 backdrop:backdrop-blur-[2px] dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100"
  onclose={closed}
  oncancel={(e) => running && e.preventDefault()}
>
  {#if report}
    <div class="flex items-center gap-3">
      <span
        class="flex size-9 shrink-0 items-center justify-center rounded-full bg-red-50 text-red-600 dark:bg-red-950/50 dark:text-red-400"
      >
        <Icon name="exclamation-triangle" class="size-5" />
      </span>
      <h2 id="cleanup-dialog-title" class="text-base font-semibold">{t('cleanup.title')}</h2>
    </div>

    {#if phase === 'done'}
      <p class="mt-3 text-sm text-zinc-600 dark:text-zinc-300">
        {t('cleanup.done', {
          ok: formatCount(summary.ok, getLocale()),
          failed: formatCount(summary.failed, getLocale()),
          size: formatBytes(summary.freed, getUnit()),
        })}
      </p>
      {#if cancelled}
        <p class="mt-1 text-sm text-amber-700 dark:text-amber-400">{t('cleanup.cancelled')}</p>
      {/if}
      {#if failures.length > 0}
        <p class="mt-3 text-xs font-medium text-red-700 dark:text-red-300">{t('cleanup.failedItems')}</p>
        <ul
          class="mt-1 max-h-[40vh] divide-y divide-zinc-100 overflow-y-auto rounded-lg border border-zinc-200 dark:divide-zinc-800 dark:border-zinc-800"
        >
          {#each failures as r (r.id)}
            <li class="px-3 py-2">
              <div class="truncate text-sm" title={r.name}>{r.name}</div>
              <div class="mt-0.5 text-xs break-all text-red-600 dark:text-red-400">{r.error}</div>
            </li>
          {/each}
        </ul>
      {/if}
    {:else}
      <p class="mt-3 text-sm text-zinc-600 dark:text-zinc-300">
        {t('cleanup.body', {
          count: formatCount(entries.length, getLocale()),
          size: formatBytes(listedSize, getUnit()),
        })}
      </p>
      {#if report.Count > entries.length}
        <p class="mt-1 text-xs text-zinc-500 dark:text-zinc-400">
          {t('cleanup.moreNotListed', { n: formatCount(report.Count - entries.length, getLocale()) })}
        </p>
      {/if}

      <div class="mt-3 flex justify-end">
        <Button variant="ghost" size="sm" disabled={running} onclick={toggleAll}>
          {t(allSelected ? 'cleanup.selectNone' : 'cleanup.selectAll')}
        </Button>
      </div>
      <ul class="mt-1 max-h-[40vh] overflow-y-auto rounded-lg border border-zinc-200 py-1 dark:border-zinc-800">
        {#each entries as e (e.ID)}
          {@const label = adviceLabel(e.Advice, e.Reason, t)}
          <li>
            <label
              class="flex items-center gap-2 px-3 py-1.5 {running
                ? ''
                : 'cursor-pointer hover:bg-zinc-100 dark:hover:bg-zinc-800'}"
            >
              <input
                type="checkbox"
                class="size-4 shrink-0 accent-emerald-600"
                checked={selected.has(e.ID)}
                disabled={running}
                onchange={() => (selected = toggleID(selected, e.ID))}
              />
              <span class="min-w-0 flex-1">
                <span class="block truncate text-sm" title={e.Name}>{e.Name}</span>
                <span class="block truncate text-xs text-zinc-400 dark:text-zinc-500" title={e.RelPath}>{e.RelPath}</span>
              </span>
              {#if label}
                <span
                  role="img"
                  class="shrink-0 text-emerald-600 dark:text-emerald-400"
                  title={label}
                  aria-label={label}
                >
                  <Icon name="sparkles" class="size-3.5" />
                </span>
              {/if}
              <span class="shrink-0 text-xs font-medium tabular-nums">{formatBytes(e.Size, getUnit())}</span>
            </label>
          </li>
        {/each}
      </ul>
    {/if}

    <div class="mt-4 flex items-center justify-between gap-3">
      {#if running}
        <span class="flex min-w-0 items-center gap-2 text-xs text-zinc-600 dark:text-zinc-300">
          <Icon name="spinner" class="size-3.5" />
          <span class="truncate">{t('cleanup.progress', { done: results.length, total, name: current })}</span>
        </span>
      {:else if phase === 'idle'}
        <span class="text-xs text-zinc-500 tabular-nums dark:text-zinc-400">
          {t('cleanup.selected', {
            count: formatCount(selection.count, getLocale()),
            size: formatBytes(selection.size, getUnit()),
          })}
        </span>
      {:else}
        <span></span>
      {/if}
      <div class="flex shrink-0 gap-2">
        {#if phase === 'done'}
          <Button variant="primary" onclick={() => dlg?.close()}>{t('common.close')}</Button>
        {:else}
          <Button disabled={cancelRequested} icon={cancelRequested ? 'spinner' : undefined} onclick={cancel}>
            {t('cleanup.cancel')}
          </Button>
          <Button variant="danger" icon="trash" disabled={running || selection.count === 0} onclick={start}>
            {t('cleanup.start')}
          </Button>
        {/if}
      </div>
    </div>
  {/if}
</dialog>
