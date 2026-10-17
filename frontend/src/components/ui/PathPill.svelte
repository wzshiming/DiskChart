<script lang="ts">
  import Icon from './Icon.svelte'

  let { path, class: cls = 'max-w-[40vw]' }: { path: string; class?: string } = $props()
</script>

<span
  class="inline-flex h-7 min-w-0 items-center gap-1.5 rounded-full border border-zinc-200 bg-zinc-100 px-2.5 font-mono text-xs text-zinc-600 dark:border-zinc-800 dark:bg-zinc-900 dark:text-zinc-300 {cls}"
  title={path}
>
  <Icon name="folder-open" class="size-3.5 text-zinc-400 dark:text-zinc-500" />
  <!-- rtl keeps the end of the path visible; LRMs stop leading/trailing slashes from flipping -->
  <span class="min-w-0 truncate" style="direction: rtl; text-align: left">{`\u200e${path}\u200e`}</span>
</span>
