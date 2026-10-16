<script module lang="ts">
  const paths = {
    'hard-drive': [
      'M5 13h14a2 2 0 0 1 2 2v3a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-3a2 2 0 0 1 2-2z',
      'M3.4 13.9 6.6 5.4A2 2 0 0 1 8.5 4h7a2 2 0 0 1 1.9 1.4l3.2 8.5',
      'M17 16.5h.01',
      'M6.5 16.5h5',
    ],
    'folder-open': [
      'M3 18.5V6.5A1.5 1.5 0 0 1 4.5 5h4.4l2 2h7.6A1.5 1.5 0 0 1 20 8.5v1.9',
      'M3 18.5 5.6 11.4A1.5 1.5 0 0 1 7 10.4h13.5a1 1 0 0 1 .94 1.34l-2.4 7.25A1.5 1.5 0 0 1 17.6 20H4.5A1.5 1.5 0 0 1 3 18.5z',
    ],
    'chevron-left': ['M15 18 9 12l6-6'],
    'chevron-right': ['M9 6l6 6-6 6'],
    'arrow-path': ['M20.5 12a8.5 8.5 0 1 1-2.5-6l2.5 2.5', 'M20.5 3.5v5h-5'],
    check: ['M5 12.5l4.5 4.5L19 7'],
    'exclamation-triangle': [
      'M10.3 4.2 2.6 17.5A2 2 0 0 0 4.3 20.5h15.4a2 2 0 0 0 1.7-3L13.7 4.2a2 2 0 0 0-3.4 0z',
      'M12 9.5v4',
      'M12 17h.01',
    ],
    trash: [
      'M4 7h16',
      'M9 7V4.5a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1V7',
      'M6 7l1 12.1a1.5 1.5 0 0 0 1.5 1.4h7a1.5 1.5 0 0 0 1.5-1.4L18 7',
      'M10 11v5.5',
      'M14 11v5.5',
    ],
    clipboard: [
      'M9 3h6a1 1 0 0 1 1 1v2a1 1 0 0 1-1 1H9a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1z',
      'M8 5H6.5A1.5 1.5 0 0 0 5 6.5v13A1.5 1.5 0 0 0 6.5 21h11a1.5 1.5 0 0 0 1.5-1.5v-13A1.5 1.5 0 0 0 17.5 5H16',
    ],
    'arrow-top-right-on-square': [
      'M13.5 4.5h6v6',
      'M19.5 4.5 11 13',
      'M17 13.5V18a1.5 1.5 0 0 1-1.5 1.5h-10A1.5 1.5 0 0 1 4 18V8a1.5 1.5 0 0 1 1.5-1.5H10',
    ],
    stop: ['M8 6h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2z'],
    spinner: ['M21 12a9 9 0 0 0-9-9'],
  } satisfies Record<string, string[]>

  export type IconName = keyof typeof paths
</script>

<script lang="ts">
  interface Props {
    name: IconName
    class?: string
  }
  let { name, class: cls = 'size-4' }: Props = $props()
</script>

<svg
  class="shrink-0 {cls} {name === 'spinner' ? 'animate-spin' : ''}"
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  stroke-width="1.75"
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
>
  {#if name === 'spinner'}
    <circle cx="12" cy="12" r="9" opacity="0.25" />
  {/if}
  {#each paths[name] as d (d)}
    <path {d} />
  {/each}
</svg>
