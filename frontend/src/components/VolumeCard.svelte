<script lang="ts">
  import type { app } from '../../wailsjs/go/models'
  import { formatBytes, formatPercent } from '../lib/format'
  import { t } from '../lib/i18n'
  import { getUnit } from '../lib/prefs.svelte'
  import Icon from './ui/Icon.svelte'

  interface Props {
    volume: app.Volume
    onclick: () => void
  }
  let { volume, onclick }: Props = $props()

  const ratio = $derived(volume.Total > 0 ? Math.min(1, volume.Used / volume.Total) : 0)
  const full = $derived(ratio > 0.9)
</script>

<button
  type="button"
  class="group relative flex flex-col gap-4 rounded-2xl border border-zinc-200 bg-white p-5 text-left shadow-sm transition hover:-translate-y-0.5 hover:border-blue-400 hover:shadow-md focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 dark:border-zinc-800 dark:bg-zinc-900 dark:hover:border-blue-500"
  {onclick}
>
  <div class="flex items-center gap-3">
    <span
      class="flex size-10 shrink-0 items-center justify-center rounded-xl bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300"
    >
      <Icon name="hard-drive" class="size-5" />
    </span>
    <div class="min-w-0 flex-1">
      <div class="truncate font-semibold">{volume.Label || t('start.systemDisk')}</div>
      <div class="truncate font-mono text-xs text-zinc-500 dark:text-zinc-400" title={volume.Mountpoint}>
        {volume.Mountpoint}
      </div>
    </div>
    <Icon
      name="chevron-right"
      class="size-5 text-blue-500 opacity-0 transition group-hover:opacity-100 group-focus-visible:opacity-100"
    />
  </div>
  <div class="h-2 overflow-hidden rounded-full bg-zinc-200 dark:bg-zinc-800">
    <div
      class="h-full rounded-full bg-gradient-to-r transition-[width] duration-500 {full
        ? 'from-amber-500 to-red-500'
        : 'from-blue-500 to-indigo-500'}"
      style:width="{(ratio * 100).toFixed(1)}%"
    ></div>
  </div>
  <div class="flex flex-col gap-1">
    <div class="flex items-center justify-between gap-2">
      <span class="min-w-0 truncate text-sm text-zinc-600 tabular-nums dark:text-zinc-300">
        {t('start.used')}
        <span class="font-semibold text-zinc-900 dark:text-zinc-100">{formatBytes(volume.Used, getUnit())}</span>
        {t('start.of')}
        {formatBytes(volume.Total, getUnit())}
      </span>
      <span
        class="shrink-0 rounded-md px-1.5 py-0.5 text-xs font-medium tabular-nums {full
          ? 'bg-red-50 text-red-700 dark:bg-red-950/40 dark:text-red-300'
          : 'bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300'}"
      >
        {formatPercent(volume.Used, volume.Total)}
      </span>
    </div>
    <div class="text-xs text-zinc-500 tabular-nums dark:text-zinc-400">
      {t('start.free')}
      {formatBytes(volume.Free, getUnit())}
    </div>
  </div>
</button>
