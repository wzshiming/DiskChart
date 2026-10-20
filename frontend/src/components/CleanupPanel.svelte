<script lang="ts">
  import type { app } from '../../wailsjs/go/models'
  import { formatBytes, formatCount } from '../lib/format'
  import { getLocale, t } from '../lib/i18n'
  import { getUnit } from '../lib/prefs.svelte'
  import TopList from './TopList.svelte'
  import Button from './ui/Button.svelte'
  import Icon from './ui/Icon.svelte'

  interface Props {
    report: app.CleanupReport | null
    selectedID: number
    onPick: (e: app.Entry) => void
    onCleanAll: () => void
  }
  let { report, selectedID, onPick, onCleanAll }: Props = $props()
</script>

{#if report === null}
  <p class="p-4 text-sm text-zinc-500">{t('common.loading')}</p>
{:else if report.Entries.length === 0}
  <p class="p-4 text-sm text-zinc-500">{t('cleanup.empty')}</p>
{:else}
  <div
    class="sticky top-0 z-10 flex items-center gap-2 border-b border-zinc-100 bg-white px-3 py-2 dark:border-zinc-800 dark:bg-zinc-900"
  >
    <Icon name="sparkles" class="size-4 text-emerald-600 dark:text-emerald-400" />
    <span class="min-w-0 flex-1 text-xs text-zinc-600 dark:text-zinc-300">
      {t('cleanup.summary', {
        size: formatBytes(report.Total, getUnit()),
        count: formatCount(report.Count, getLocale()),
      })}
    </span>
    <Button variant="danger-outline" size="sm" icon="trash" onclick={onCleanAll}>{t('cleanup.cleanAll')}</Button>
  </div>
  <TopList entries={report.Entries} {selectedID} hint={t('cleanup.hint')} {onPick} />
{/if}
