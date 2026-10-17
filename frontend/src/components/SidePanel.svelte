<script lang="ts">
  import type { app } from '../../wailsjs/go/models'
  import { t } from '../lib/i18n'
  import ErrorsPanel from './ErrorsPanel.svelte'
  import TopList from './TopList.svelte'
  import TypesPanel from './TypesPanel.svelte'

  interface Props {
    topFiles: app.Entry[] | null
    topDirs: app.Entry[] | null
    extensions: app.ExtReport | null
    errors: app.ScanError[] | null
    dark: boolean
    selectedID: number
    onPickFile: (e: app.Entry) => void
    onPickDir: (e: app.Entry) => void
  }
  let { topFiles, topDirs, extensions, errors, dark, selectedID, onPickFile, onPickDir }: Props = $props()

  type Tab = 'files' | 'dirs' | 'types' | 'errors'
  let tab = $state<Tab>('files')
  const tabs: Tab[] = ['files', 'dirs', 'types', 'errors']
</script>

<div
  class="flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900"
>
  <div class="flex shrink-0 gap-1 border-b border-zinc-200 px-2 pt-1.5 dark:border-zinc-800" role="tablist">
    {#each tabs as id (id)}
      <button
        type="button"
        role="tab"
        aria-selected={tab === id}
        class="-mb-px flex items-center gap-1.5 border-b-2 px-2 py-1.5 text-xs font-medium whitespace-nowrap {tab === id
          ? 'border-blue-500 text-zinc-900 dark:text-zinc-100'
          : 'border-transparent text-zinc-500 hover:text-zinc-800 dark:text-zinc-400 dark:hover:text-zinc-200'}"
        onclick={() => (tab = id)}
      >
        {t('results.tabs.' + id)}
        {#if id === 'errors' && errors && errors.length > 0}
          <span
            class="rounded-full bg-red-100 px-1.5 text-[10px] leading-4 text-red-700 tabular-nums dark:bg-red-950 dark:text-red-300"
          >
            {errors.length >= 500 ? '500+' : errors.length}
          </span>
        {/if}
      </button>
    {/each}
  </div>
  <div class="min-h-0 flex-1 overflow-y-auto" role="tabpanel">
    {#if tab === 'files'}
      <TopList entries={topFiles} {selectedID} onPick={onPickFile} />
    {:else if tab === 'dirs'}
      <TopList entries={topDirs} {selectedID} hint={t('results.dominanceHint')} onPick={onPickDir} />
    {:else if tab === 'types'}
      <TypesPanel report={extensions} {dark} />
    {:else}
      <ErrorsPanel {errors} />
    {/if}
  </div>
</div>
