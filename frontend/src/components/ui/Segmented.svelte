<script lang="ts">
  interface Props {
    options: { value: string; label: string; title?: string }[]
    value: string
    onChange: (v: string) => void
    ariaLabel: string
  }
  let { options, value, onChange, ariaLabel }: Props = $props()

  const segBtn =
    'rounded-md px-3 py-1 text-xs font-medium transition-colors focus-visible:outline-2 focus-visible:outline-blue-500'
</script>

<div class="flex shrink-0 rounded-lg bg-zinc-100 p-0.5 dark:bg-zinc-800" role="group" aria-label={ariaLabel}>
  {#each options as o (o.value)}
    <button
      type="button"
      aria-pressed={value === o.value}
      title={o.title}
      class="{segBtn} {value === o.value
        ? 'bg-white text-zinc-900 shadow-sm dark:bg-zinc-700 dark:text-zinc-100'
        : 'text-zinc-500 hover:text-zinc-800 dark:text-zinc-400 dark:hover:text-zinc-200'}"
      onclick={() => onChange(o.value)}
    >
      {o.label}
    </button>
  {/each}
</div>
