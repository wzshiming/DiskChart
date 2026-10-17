<script lang="ts">
  import { onDestroy } from 'svelte'
  import { api } from '../lib/api'
  import { formatBytes, formatCount, formatDate } from '../lib/format'
  import { getLocale, t } from '../lib/i18n'
  import { getUnit } from '../lib/prefs.svelte'
  import type { Item } from '../lib/state'
  import Button from './ui/Button.svelte'

  interface Props {
    item: Item | null
    path: string
    isRoot: boolean
    onDelete: () => void
  }
  let { item, path, isRoot, onDelete }: Props = $props()

  let copied = $state(false)
  let error = $state('')
  let timer: ReturnType<typeof setTimeout> | undefined

  $effect(() => {
    void item?.id
    error = ''
  })

  async function run(fn: () => Promise<void>) {
    error = ''
    try {
      await fn()
    } catch (e) {
      error = String(e)
    }
  }

  function copy() {
    if (!item) return
    const id = item.id
    run(async () => {
      await api.copyPath(id)
      copied = true
      clearTimeout(timer)
      timer = setTimeout(() => (copied = false), 1500)
    })
  }

  onDestroy(() => clearTimeout(timer))
</script>

<div class="shrink-0 rounded-xl border border-zinc-200 bg-white p-3 dark:border-zinc-800 dark:bg-zinc-900">
  {#if item}
    <div class="truncate text-sm font-semibold" title={item.name}>{item.name}</div>
    <dl class="mt-2 grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-xs">
      <dt class="text-zinc-500 dark:text-zinc-400">{t('detail.path')}</dt>
      <dd class="font-mono break-all">{path}</dd>
      <dt class="text-zinc-500 dark:text-zinc-400">{t('detail.size')}</dt>
      <dd class="tabular-nums">{formatBytes(item.size, getUnit())}</dd>
      {#if item.isDir}
        <dt class="text-zinc-500 dark:text-zinc-400">{t('detail.files')}</dt>
        <dd class="tabular-nums">{formatCount(item.fileCount, getLocale())}</dd>
      {:else if item.modTime > 0}
        <dt class="text-zinc-500 dark:text-zinc-400">{t('detail.modified')}</dt>
        <dd>{formatDate(item.modTime, getLocale())}</dd>
      {/if}
    </dl>
    <div class="mt-3 flex flex-wrap gap-1">
      <Button
        variant="secondary"
        size="sm"
        icon="arrow-top-right-on-square"
        onclick={() => item && run(() => api.reveal(item.id))}
      >
        {t('detail.reveal')}
      </Button>
      <Button variant="secondary" size="sm" icon={copied ? 'check' : 'clipboard'} onclick={copy}>
        {copied ? t('detail.copied') : t('detail.copy')}
      </Button>
      <Button variant="danger-outline" size="sm" icon="trash" class="ml-auto" disabled={isRoot} onclick={onDelete}>
        {t('detail.delete')}
      </Button>
    </div>
    {#if error}
      <p class="mt-2 text-xs break-all text-red-600 dark:text-red-400">{t('common.error')}: {error}</p>
    {/if}
  {:else}
    <p class="text-sm text-zinc-500">{t('common.loading')}</p>
  {/if}
</div>
