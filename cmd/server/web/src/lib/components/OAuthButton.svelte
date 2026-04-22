<script lang="ts">
  import GoogleIcon from "./ui/GoogleIcon.svelte"

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
  <span class="flex w-[200px] text-left gap-3">
    {#if provider === 'google'}
      <GoogleIcon />
    {:else if provider === 'canva'}
      <img src="/canva.logo.svg" width="20" alt="canva logo" />
    {:else}
      <svg class="w-5 h-5 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
        <circle cx="12" cy="12" r="10"/>
        <path d="M12 8v4l3 3"/>
      </svg>
    {/if}
    {label}
  </span>
</a>
