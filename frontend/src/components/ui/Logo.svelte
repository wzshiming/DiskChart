<script lang="ts">
  let { class: cls = 'size-4' }: { class?: string } = $props()
</script>

<svg class={cls} viewBox="0 0 24 24" fill="none" aria-hidden="true">
  <rect x="3" y="3" width="10" height="18" rx="1.5" fill="currentColor" />
  <rect x="14.5" y="3" width="6.5" height="8" rx="1.5" fill="currentColor" opacity="0.75" />
  <rect x="14.5" y="12.5" width="6.5" height="8.5" rx="1.5" fill="currentColor" opacity="0.5" />
</svg>
