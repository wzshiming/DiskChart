<script lang="ts">
  import { untrack } from 'svelte'
  import type { app } from '../../wailsjs/go/models'
  import { api } from '../lib/api'
  import { formatBytes, formatCount } from '../lib/format'
  import { getLocale, t } from '../lib/i18n'
  import { getUnit } from '../lib/prefs.svelte'
  import TopList from './TopList.svelte'
  import Button from './ui/Button.svelte'

  interface Target {
    parentID: number
    skip: number
    count: number
    size: number
  }

  interface Props {
    target: Target | null
    selectedID: number
    onClose: () => void
    onPick: (e: app.Entry) => void
  }
  let { target, selectedID, onClose, onPick }: Props = $props()

  const PAGE = 200

  let dlg = $state<HTMLDialogElement>()
  let entries = $state.raw<app.Entry[] | null>(null)
  let loaded = $state(0)
  let hasMore = $state(false)
  let busy = $state(false)
  let error = $state('')

  let shown: Target | null = null
  // Bumped per request and on close so only the latest request's response is applied.
  let seq = 0

  $effect(() => {
    const tg = target
    if (!dlg) return
    if (tg && tg !== shown) {
      shown = tg
      entries = null
      loaded = 0
      hasMore = false
      error = ''
      if (!dlg.open) dlg.showModal()
      untrack(() => load(0))
    } else if (!tg) {
      shown = null
      seq++
      busy = false
      if (dlg.open) dlg.close()
    }
  })

  function load(offset: number) {
    const tg = shown
    if (!tg) return
    const n = ++seq
    busy = true
    error = ''
    api
      .children(tg.parentID, tg.skip + offset, PAGE)
      .then((page) => {
        if (n !== seq) return
        const got = page ?? []
        entries = offset === 0 ? got : [...(entries ?? []), ...got]
        loaded = entries.length
        hasMore = got.length === PAGE && loaded < tg.count
      })
      .catch((e) => {
        if (n === seq) error = String(e)
      })
      .finally(() => {
        if (n === seq) busy = false
      })
  }
</script>

<dialog
  bind:this={dlg}
  aria-labelledby="aggregate-dialog-title"
  aria-describedby="aggregate-dialog-hint"
  class="m-auto w-[min(560px,92vw)] rounded-2xl border border-zinc-200 bg-white p-6 text-zinc-900 shadow-2xl backdrop:bg-black/40 backdrop:backdrop-blur-[2px] dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100"
  onclose={onClose}
>
  {#if target}
    <div class="flex items-start gap-3">
      <div class="min-w-0 flex-1">
        <h2 id="aggregate-dialog-title" class="text-base font-semibold">
          {t('results.aggregate', { count: formatCount(target.count, getLocale()) })}
        </h2>
        <p class="text-sm text-zinc-600 tabular-nums dark:text-zinc-300">{formatBytes(target.size, getUnit())}</p>
        <p id="aggregate-dialog-hint" class="mt-1 text-xs text-zinc-500 dark:text-zinc-400">{t('aggregate.hint')}</p>
      </div>
      <Button variant="ghost" size="sm" ariaLabel={t('common.close')} onclick={() => dlg?.close()}>
        {t('common.close')}
      </Button>
    </div>

    {#if error}
      <div
        class="mt-3 flex items-center gap-2 rounded-md bg-red-50 px-3 py-2 text-xs text-red-700 dark:bg-red-950/50 dark:text-red-300"
        role="alert"
      >
        <span class="min-w-0 flex-1 break-all"><span class="font-medium">{t('common.error')}</span>: {error}</span>
        <Button size="sm" icon="arrow-path" disabled={busy} onclick={() => load(loaded)}>{t('common.retry')}</Button>
      </div>
    {/if}

    <div
      class="mt-3 max-h-[60vh] overflow-y-auto rounded-lg border border-zinc-200 dark:border-zinc-800"
    >
      {#if !(error && entries === null)}
        <TopList {entries} {selectedID} {onPick} />
      {/if}
    </div>

    <div class="mt-3 flex items-center justify-between gap-3">
      <span class="text-xs text-zinc-500 tabular-nums dark:text-zinc-400">
        {t('aggregate.loaded', {
          loaded: formatCount(loaded, getLocale()),
          total: formatCount(target.count, getLocale()),
        })}
      </span>
      {#if hasMore}
        <Button size="sm" icon={busy ? 'spinner' : undefined} disabled={busy} onclick={() => load(loaded)}>
          {t('aggregate.loadMore')}
        </Button>
      {/if}
    </div>
  {/if}
</dialog>
