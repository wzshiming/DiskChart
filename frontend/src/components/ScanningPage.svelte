<script lang="ts">
  import type { app } from '../../wailsjs/go/models'
  import { api } from '../lib/api'
  import { formatBytes, formatCount, formatDuration } from '../lib/format'
  import { getLocale, t } from '../lib/i18n'
  import Button from './ui/Button.svelte'
  import Icon from './ui/Icon.svelte'
  import PathPill from './ui/PathPill.svelte'

  interface Props {
    root: string
    progress: app.Progress | null
    platform: string
  }
  let { root, progress, platform }: Props = $props()

  let cancelling = $state(false)
  let cancelError = $state('')
  const locale = getLocale()

  const stats = $derived([
    { label: t('scanning.files'), value: formatCount(progress?.Files ?? 0, locale) },
    { label: t('scanning.dirs'), value: formatCount(progress?.Dirs ?? 0, locale) },
    { label: t('scanning.bytes'), value: formatBytes(progress?.Bytes ?? 0, platform) },
    { label: t('scanning.errors'), value: formatCount(progress?.Errors ?? 0, locale) },
  ])

  function cancel() {
    cancelling = true
    cancelError = ''
    api.cancelScan().catch((e) => {
      cancelling = false
      cancelError = String(e)
    })
  }
</script>

<div class="flex h-full items-center justify-center p-8">
  <div class="w-full max-w-2xl">
    <div class="flex items-center gap-3">
      <Icon name="spinner" class="size-7 text-blue-500" />
      <h1 class="text-3xl font-semibold tracking-tight">{t('scanning.title')}</h1>
    </div>
    <PathPill path={root} class="mt-3 max-w-full" />

    <div class="mt-8 grid grid-cols-4 gap-3">
      {#each stats as s (s.label)}
        <div
          class="rounded-2xl border border-zinc-200 bg-white px-4 py-3 shadow-sm dark:border-zinc-800 dark:bg-zinc-900"
        >
          <div class="text-xs text-zinc-500 dark:text-zinc-400">{s.label}</div>
          <div class="mt-1 text-xl font-semibold tabular-nums">{s.value}</div>
        </div>
      {/each}
    </div>

    <div class="mt-4 flex items-center gap-2 text-sm text-zinc-500 dark:text-zinc-400">
      <span>{t('scanning.elapsed')}</span>
      <span class="font-medium text-zinc-700 tabular-nums dark:text-zinc-200">
        {formatDuration(progress?.ElapsedMs ?? 0)}
      </span>
    </div>

    <div
      class="mt-2 h-5 truncate font-mono text-xs text-zinc-400 dark:text-zinc-500"
      style="direction: rtl; text-align: left"
      title={progress?.CurrentPath ?? ''}
    >
      {progress?.CurrentPath ? `\u200e${progress.CurrentPath}\u200e` : ''}
    </div>

    <Button icon="stop" class="mt-8" disabled={cancelling} onclick={cancel}>{t('scanning.cancel')}</Button>

    {#if cancelError}
      <div
        class="mt-4 flex items-start gap-2 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800 dark:border-red-900 dark:bg-red-950/40 dark:text-red-200"
        role="alert"
      >
        <Icon name="exclamation-triangle" class="mt-0.5 size-4" />
        <div class="min-w-0">
          <span class="font-medium">{t('common.error')}</span>
          <span class="ml-2 break-all">{cancelError}</span>
        </div>
      </div>
    {/if}
  </div>
</div>
