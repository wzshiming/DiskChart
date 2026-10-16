<script lang="ts">
  import type { app } from '../../wailsjs/go/models'
  import { t } from '../lib/i18n'

  interface Props {
    errors: app.ScanError[] | null
  }
  let { errors }: Props = $props()
</script>

{#if errors === null}
  <p class="p-4 text-sm text-zinc-500">{t('common.loading')}</p>
{:else if errors.length === 0}
  <p class="p-4 text-sm text-zinc-500">{t('results.noErrors')}</p>
{:else}
  <ul class="divide-y divide-zinc-100 dark:divide-zinc-800">
    {#each errors as e, i (i)}
      <li class="px-3 py-2">
        <div class="font-mono text-xs break-all">{e.Path}</div>
        <div class="mt-0.5 text-xs text-red-600 dark:text-red-400">{e.Msg}</div>
      </li>
    {/each}
  </ul>
{/if}
