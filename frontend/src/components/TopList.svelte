<script lang="ts">
  import type { app } from '../../wailsjs/go/models'
  import { formatBytes } from '../lib/format'
  import { t } from '../lib/i18n'

  interface Props {
    entries: app.Entry[] | null
    platform: string
    selectedID: number
    hint?: string
    onPick: (e: app.Entry) => void
  }
  let { entries, platform, selectedID, hint = '', onPick }: Props = $props()

  const max = $derived(entries && entries.length > 0 ? entries[0].Size : 0)
</script>

{#if entries === null}
  <p class="p-4 text-sm text-zinc-500">{t('common.loading')}</p>
{:else if entries.length === 0}
  <p class="p-4 text-sm text-zinc-500">{t('results.empty')}</p>
{:else}
  {#if hint}
    <p class="px-3 pt-2 pb-1 text-xs text-zinc-500 dark:text-zinc-400">{hint}</p>
  {/if}
  <ol class="py-1">
    {#each entries as e, i (e.ID)}
      <li>
        <button
          type="button"
          class="flex w-full items-center gap-2 px-3 py-1.5 text-left hover:bg-zinc-100 dark:hover:bg-zinc-800 {e.ID ===
          selectedID
            ? 'bg-blue-50 dark:bg-blue-950/40'
            : ''}"
          onclick={() => onPick(e)}
        >
          <span class="w-6 shrink-0 text-right text-xs text-zinc-400 tabular-nums">{i + 1}</span>
          <span class="min-w-0 flex-1">
            <span class="block truncate text-sm" title={e.Name}>{e.Name}</span>
            <span class="block truncate text-xs text-zinc-400 dark:text-zinc-500" title={e.RelPath}>{e.RelPath}</span>
            <span class="mt-1 block h-0.5 rounded-full bg-zinc-100 dark:bg-zinc-800">
              <span
                class="block h-full rounded-full bg-blue-500/70"
                style:width="{max > 0 ? ((e.Size / max) * 100).toFixed(1) : 0}%"
              ></span>
            </span>
          </span>
          <span class="shrink-0 text-xs font-medium tabular-nums">{formatBytes(e.Size, platform)}</span>
        </button>
      </li>
    {/each}
  </ol>
{/if}
