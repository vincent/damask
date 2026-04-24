<script lang="ts">
  import type { Snippet } from 'svelte'

  interface Props {
    variant?: 'primary' | 'secondary' | 'ghost' | 'danger' | 'outline'
    size?: 'sm' | 'md'
    title?: string
    loading?: boolean
    disabled?: boolean
    type?: 'button' | 'submit' | 'reset'
    onclick?: (e: MouseEvent) => void
    children?: Snippet
    icon?: Snippet
    class?: string
    style?: string
  }

  let {
    title = '',
    variant = 'primary',
    size = 'md',
    loading = false,
    disabled = false,
    type = 'button',
    onclick,
    children,
    icon,
    class: extraClass = '',
    style = '',
  }: Props = $props()

  const base =
    'inline-flex items-center justify-center gap-1.5 font-medium rounded-lg transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-1 disabled:opacity-50 disabled:cursor-not-allowed'

  const variants: Record<string, string> = {
    primary:
      'bg-indigo-600 text-white hover:bg-indigo-700 focus-visible:ring-indigo-400 dark:bg-indigo-500 dark:hover:bg-indigo-600',
    secondary:
      'bg-white text-gray-700 border border-gray-300 hover:bg-gray-50 focus-visible:ring-gray-300 dark:bg-gray-800 dark:text-gray-200 dark:border-gray-600 dark:hover:bg-gray-700',
    ghost:
      'text-gray-600 hover:bg-gray-100 focus-visible:ring-gray-300 dark:text-gray-300 dark:hover:bg-gray-700',
    danger:
      'bg-red-600 text-white hover:bg-red-700 focus-visible:ring-red-400 dark:bg-red-500 dark:hover:bg-red-600',
    outline:
      'flex items-center gap-1.5 rounded-lg border border-gray-200 px-3 py-1.5 text-md text-gray-600 transition-colors hover:border-indigo-300 hover:bg-indigo-50 hover:text-indigo-700 dark:border-gray-700 dark:text-gray-400 dark:hover:border-indigo-700 dark:hover:bg-indigo-900/20 dark:hover:text-indigo-400'

  }

  const sizes: Record<string, string> = {
    sm: 'px-2.5 py-1.5 text-sm',
    md: 'px-3.5 py-2 text-md',
  }
</script>

<button
  {type}
  {title}
  disabled={disabled || loading}
  {onclick}
  {style}
  class="{base} {variants[variant]} {sizes[size]} {extraClass}"
>
  {#if loading}
    <svg class="h-3.5 w-3.5 animate-spin" viewBox="0 0 24 24" fill="none">
      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
      <path
        class="opacity-75"
        fill="currentColor"
        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
      />
    </svg>
  {:else if icon}
    {@render icon()}
  {/if}
  {@render children?.()}
</button>
