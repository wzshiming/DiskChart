<script lang="ts">
  import type { app } from '../../wailsjs/go/models'
  import { baseName } from './item'
  import { t } from '../lib/i18n'

  interface Props {
    crumbs: app.Crumb[]
    onNavigate: (id: number) => void
  }
  let { crumbs, onNavigate }: Props = $props()

  let nav: HTMLElement | undefined = $state()
  let navWidth = $state(0)

  // The last crumb is the current folder: keep the trail's end in view, also after resizes.
  $effect(() => {
    void crumbs
    void navWidth
    nav?.scrollTo({ left: nav.scrollWidth })
  })
</script>

<nav
  bind:this={nav}
  bind:clientWidth={navWidth}
  aria-label={t('results.location')}
  class="flex min-w-0 items-center overflow-x-auto text-sm whitespace-nowrap"
>
  {#each crumbs as c, i (c.ID)}
    {#if i > 0}
      <svg class="mx-0.5 size-3.5 shrink-0 text-zinc-400" viewBox="0 0 16 16" fill="none" aria-hidden="true">
        <path d="M6 4l4 4-4 4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
      </svg>
    {/if}
    {#if i === crumbs.length - 1}
      <span class="rounded px-1.5 py-0.5 font-medium" title={i === 0 ? c.Name : undefined}>
        {i === 0 ? baseName(c.Name) : c.Name}
      </span>
    {:else}
      <button
        type="button"
        class="rounded px-1.5 py-0.5 text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900 dark:text-zinc-400 dark:hover:bg-zinc-800 dark:hover:text-zinc-100"
        title={i === 0 ? c.Name : undefined}
        onclick={() => onNavigate(c.ID)}
      >
        {i === 0 ? baseName(c.Name) : c.Name}
      </button>
    {/if}
  {/each}
</nav>
