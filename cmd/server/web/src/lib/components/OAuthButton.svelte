<script lang="ts">
  interface Props {
    provider: 'oidc' | 'google' | 'canva'
    label: string
    href: string
  }
  let { provider, label, href }: Props = $props()
</script>

<a
  {href}
  class="flex items-center justify-center gap-3 w-full px-4 py-2.5 border border-zinc-300 dark:border-zinc-600 rounded-lg bg-white dark:bg-zinc-800 text-zinc-800 dark:text-zinc-100 font-medium text-sm hover:bg-zinc-50 dark:hover:bg-zinc-700 transition-colors"
>
  {#if provider === 'google'}
    <svg class="w-5 h-5 shrink-0" viewBox="0 0 24 24" aria-hidden="true">
      <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" fill="#4285F4"/>
      <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/>
      <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l3.66-2.84z" fill="#FBBC05"/>
      <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/>
    </svg>
  {:else if provider === 'canva'}
    <svg class="w-5 h-5 shrink-0" viewBox="0 0 24 24" aria-hidden="true" fill="#7D2AE8">
      <path d="M12 2C6.477 2 2 6.477 2 12s4.477 10 10 10 10-4.477 10-10S17.523 2 12 2zm4.5 13.5c-.69 0-1.37-.26-1.88-.74l-1.24-1.16c-.3-.28-.73-.28-1.03 0l-1.24 1.16c-.51.48-1.19.74-1.88.74-1.55 0-2.81-1.26-2.81-2.81 0-.69.26-1.37.74-1.88l1.16-1.24c.28-.3.28-.73 0-1.03L7.12 8.3C6.64 7.79 6.38 7.11 6.38 6.42c0-1.55 1.26-2.81 2.81-2.81.69 0 1.37.26 1.88.74l1.24 1.16c.3.28.73.28 1.03 0l1.24-1.16c.51-.48 1.19-.74 1.88-.74 1.55 0 2.81 1.26 2.81 2.81 0 .69-.26 1.37-.74 1.88l-1.16 1.24c-.28.3-.28.73 0 1.03l1.16 1.24c.48.51.74 1.19.74 1.88 0 1.55-1.26 2.81-2.81 2.81z"/>
    </svg>
  {:else}
    <svg class="w-5 h-5 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
      <circle cx="12" cy="12" r="10"/>
      <path d="M12 8v4l3 3"/>
    </svg>
  {/if}
  {label}
</a>
