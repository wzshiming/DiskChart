<script lang="ts">
  import { t } from '../lib/i18n'
  import type { Phase } from '../lib/state'
  import Button from './ui/Button.svelte'
  import Logo from './ui/Logo.svelte'
  import PathPill from './ui/PathPill.svelte'

  interface Props {
    phase: Phase
    root: string
    summary?: { size: string; files: string } | null
    onBack: () => void
  }
  let { phase, root, summary = null, onBack }: Props = $props()

  const chip =
    'inline-flex h-7 items-center rounded-full px-2.5 text-xs text-zinc-500 tabular-nums ring-1 ring-zinc-200 ring-inset dark:text-zinc-400 dark:ring-zinc-800'
</script>

<header
  class="flex h-12 shrink-0 items-center gap-3 border-b border-zinc-200 bg-white px-3 dark:border-zinc-800 dark:bg-zinc-950"
>
  <div class="flex shrink-0 items-center gap-2">
    <span
      class="flex size-7 items-center justify-center rounded-md bg-gradient-to-br from-blue-500 to-indigo-600 text-white shadow-sm"
    >
      <Logo class="size-4" />
    </span>
    <span class="text-sm font-semibold tracking-tight">{t('app.title')}</span>
  </div>

  {#if phase !== 'start' && root}
    <span class="h-5 w-px shrink-0 bg-zinc-200 dark:bg-zinc-800" aria-hidden="true"></span>
    {#if phase === 'results'}
      <Button variant="ghost" size="sm" icon="chevron-left" onclick={onBack}>{t('common.back')}</Button>
    {/if}
    <PathPill path={root} />
  {/if}

  <span class="min-w-0 flex-1"></span>

  {#if phase === 'results'}
    {#if summary}
      <div class="hidden shrink-0 items-center gap-1.5 md:flex">
        <span class={chip}>{summary.size}</span>
        <span class={chip}>
          {summary.files === '1' ? t('results.fileCountOne') : t('results.fileCount', { count: summary.files })}
        </span>
      </div>
    {/if}
    <Button variant="primary" icon="arrow-path" onclick={onBack}>{t('results.newScan')}</Button>
  {/if}
</header>
