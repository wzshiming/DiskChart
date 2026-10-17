<script lang="ts">
  import { api } from '../lib/api'
  import { formatBytes, formatCount } from '../lib/format'
  import { getLocale, t } from '../lib/i18n'
  import { getUnit } from '../lib/prefs.svelte'
  import type { Item } from '../lib/state'
  import Button from './ui/Button.svelte'
  import Icon from './ui/Icon.svelte'

  interface Props {
    item: Item | null
    path: string
    onClose: () => void
    onDeleted: () => void
    onStale: () => void
  }
  let { item, path, onClose, onDeleted, onStale }: Props = $props()

  let dlg = $state<HTMLDialogElement>()
  let busy = $state(false)
  let error = $state('')

  $effect(() => {
    if (item && dlg && !dlg.open) {
      error = ''
      busy = false
      dlg.showModal()
    } else if (!item && dlg?.open) {
      dlg.close()
    }
  })

  async function confirm() {
    if (!item) return
    busy = true
    error = ''
    try {
      await api.delete(item.id)
      onDeleted()
    } catch (e) {
      error = String(e)
      const s = await api.state().catch(() => null)
      if (s?.TreeStale) onStale()
    } finally {
      busy = false
    }
  }
</script>

<dialog
  bind:this={dlg}
  aria-labelledby="delete-dialog-title"
  class="m-auto w-[min(440px,90vw)] rounded-2xl border border-zinc-200 bg-white p-6 text-zinc-900 shadow-2xl backdrop:bg-black/40 backdrop:backdrop-blur-[2px] dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100"
  onclose={onClose}
  oncancel={(e) => busy && e.preventDefault()}
>
  {#if item}
    {@const files = item.isDir ? item.fileCount : 1}
    <div class="flex items-center gap-3">
      <span
        class="flex size-9 shrink-0 items-center justify-center rounded-full bg-red-50 text-red-600 dark:bg-red-950/50 dark:text-red-400"
      >
        <Icon name="exclamation-triangle" class="size-5" />
      </span>
      <h2 id="delete-dialog-title" class="text-base font-semibold">{t('delete.title')}</h2>
    </div>
    <p class="mt-3 text-sm [overflow-wrap:anywhere] text-zinc-600 dark:text-zinc-300">
      {t(files === 1 ? 'delete.bodyOne' : 'delete.body', {
        path,
        size: formatBytes(item.size, getUnit()),
        files: formatCount(files, getLocale()),
      })}
    </p>
    {#if error}
      <p class="mt-3 rounded-md bg-red-50 px-3 py-2 text-xs break-all text-red-700 dark:bg-red-950/50 dark:text-red-300">
        <span class="font-medium">{t('delete.failed')}</span>: {error}
      </p>
    {/if}
    <div class="mt-5 flex justify-end gap-2">
      <Button disabled={busy} onclick={() => dlg?.close()}>{t('delete.cancel')}</Button>
      <Button variant="danger" icon={busy ? 'spinner' : 'trash'} disabled={busy} onclick={confirm}>
        {t('delete.confirm')}
      </Button>
    </div>
  {/if}
</dialog>
