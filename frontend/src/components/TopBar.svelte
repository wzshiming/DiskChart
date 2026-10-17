<script lang="ts">
  import type { app } from '../../wailsjs/go/models'
  import appicon from '../../../build/appicon.png'
  import type { Unit } from '../lib/format'
  import { getLocale, setLocale, t, type Locale } from '../lib/i18n'
  import { getUnit, setUnit } from '../lib/prefs.svelte'
  import type { Phase } from '../lib/state'
  import Breadcrumbs from './Breadcrumbs.svelte'
  import Button from './ui/Button.svelte'
  import PathPill from './ui/PathPill.svelte'
  import Segmented from './ui/Segmented.svelte'

  interface Props {
    phase: Phase
    root: string
    crumbs: app.Crumb[]
    partial: boolean
    summary?: { size: string; files: string } | null
    onNavigate: (id: number) => void
    onBack: () => void
  }
  let { phase, root, crumbs, partial, summary = null, onNavigate, onBack }: Props = $props()

  const chip =
    'inline-flex h-7 items-center rounded-full px-2.5 text-xs text-zinc-500 tabular-nums ring-1 ring-zinc-200 ring-inset dark:text-zinc-400 dark:ring-zinc-800'
</script>

<header
  class="flex h-12 shrink-0 items-center gap-3 border-b border-zinc-200 bg-white px-3 dark:border-zinc-800 dark:bg-zinc-950"
>
  <div class="flex shrink-0 items-center gap-2">
    <img src={appicon} alt="" draggable="false" class="size-7 shrink-0 select-none dark:invert" />
    <span class="text-sm font-semibold tracking-tight">{t('app.title')}</span>
  </div>

  <div class="flex min-w-0 flex-1 items-center gap-3">
    {#if phase === 'start'}
      <span class="h-5 w-px shrink-0 bg-zinc-200 dark:bg-zinc-800" aria-hidden="true"></span>
      <span class="truncate text-sm text-zinc-500 dark:text-zinc-400">{t('app.subtitle')}</span>
    {:else if phase === 'scanning' && root}
      <span class="h-5 w-px shrink-0 bg-zinc-200 dark:bg-zinc-800" aria-hidden="true"></span>
      <PathPill path={root} />
    {:else if phase === 'results'}
      <span class="h-5 w-px shrink-0 bg-zinc-200 dark:bg-zinc-800" aria-hidden="true"></span>
      <Button variant="ghost" size="sm" icon="chevron-left" onclick={onBack}>{t('common.back')}</Button>
      <div class="min-w-0 flex-1"><Breadcrumbs {crumbs} {onNavigate} /></div>
      {#if partial}
        <span
          class="shrink-0 rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800 dark:bg-amber-950 dark:text-amber-300"
        >
          {t('results.partial')}
        </span>
      {/if}
    {/if}
  </div>

  {#if phase === 'results' && summary}
    <div class="hidden shrink-0 items-center gap-1.5 md:flex">
      <span class={chip}>{summary.size}</span>
      <span class={chip}>
        {summary.files === '1' ? t('results.fileCountOne') : t('results.fileCount', { count: summary.files })}
      </span>
    </div>
  {/if}

  <div class="flex shrink-0 items-center gap-1.5">
    <Segmented
      options={[
        { value: 'si', label: 'KB', title: t('prefs.unitSi') },
        { value: 'iec', label: 'KiB', title: t('prefs.unitIec') },
      ]}
      value={getUnit()}
      onChange={(v) => setUnit(v as Unit)}
      ariaLabel={t('prefs.unit')}
    />
    <Segmented
      options={[
        { value: 'zh', label: '中' },
        { value: 'en', label: 'EN' },
      ]}
      value={getLocale()}
      onChange={(v) => setLocale(v as Locale)}
      ariaLabel={t('prefs.language')}
    />
  </div>
</header>
