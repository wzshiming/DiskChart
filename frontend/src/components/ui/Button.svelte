<script lang="ts">
  import type { Snippet } from 'svelte'
  import Icon, { type IconName } from './Icon.svelte'

  type Variant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'danger-outline' | 'warning'

  interface Props {
    variant?: Variant
    size?: 'sm' | 'md'
    icon?: IconName
    iconRight?: IconName
    disabled?: boolean
    type?: 'button' | 'submit' | 'reset'
    onclick?: (e: MouseEvent) => void
    class?: string
    title?: string
    ariaLabel?: string
    children?: Snippet
  }
  let {
    variant = 'secondary',
    size = 'md',
    icon,
    iconRight,
    disabled = false,
    type = 'button',
    onclick,
    class: cls = '',
    title,
    ariaLabel,
    children,
  }: Props = $props()

  const variants: Record<Variant, string> = {
    primary: 'bg-blue-600 text-white shadow-sm hover:bg-blue-700 dark:bg-blue-500 dark:hover:bg-blue-400',
    secondary:
      'border border-zinc-300 bg-white text-zinc-800 shadow-sm hover:bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100 dark:hover:bg-zinc-800',
    ghost: 'text-zinc-700 hover:bg-zinc-100 dark:text-zinc-300 dark:hover:bg-zinc-800',
    danger: 'bg-red-600 text-white shadow-sm hover:bg-red-700',
    'danger-outline':
      'border border-red-300 text-red-600 hover:bg-red-50 dark:border-red-800 dark:text-red-400 dark:hover:bg-red-950/40',
    warning: 'bg-amber-500 text-white shadow-sm hover:bg-amber-600',
  }

  const sizes = $derived(
    size === 'sm'
      ? { box: children ? 'h-7 px-2.5 text-xs' : 'size-7 text-xs', icon: 'size-3.5' }
      : { box: children ? 'h-9 px-3.5 text-sm' : 'size-9 text-sm', icon: 'size-4' },
  )
</script>

<button
  {type}
  {disabled}
  {title}
  aria-label={ariaLabel}
  class="inline-flex shrink-0 items-center justify-center gap-1.5 rounded-lg font-medium whitespace-nowrap transition select-none focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-50 disabled:active:scale-100 {sizes.box} {variants[
    variant
  ]} {cls}"
  {onclick}
>
  {#if icon}<Icon name={icon} class={sizes.icon} />{/if}
  {@render children?.()}
  {#if iconRight}<Icon name={iconRight} class={sizes.icon} />{/if}
</button>
