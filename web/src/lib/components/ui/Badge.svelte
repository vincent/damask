<script lang="ts">
  import type { Snippet } from 'svelte'

  interface Props {
    variant?: 'image' | 'video' | 'audio' | 'document' | 'neutral'
    size?: 'sm' | 'md'
    children: Snippet
  }

  let { variant = 'neutral', size = 'sm', children }: Props = $props()

  const variants: Record<string, string> = {
    image:
      'bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300',
    video:
      'bg-red-100 text-red-700 dark:bg-red-900/50 dark:text-red-300',
    audio:
      'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/50 dark:text-emerald-300',
    document:
      'bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300',
    neutral:
      'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300',
  }

  const sizes: Record<string, string> = {
    sm: 'px-1.5 py-0.5 text-[11px]',
    md: 'px-2 py-1 text-sm',
  }
</script>

<span class="inline-flex items-center rounded font-medium {variants[variant]} {sizes[size]}">
  {@render children()}
</span>
