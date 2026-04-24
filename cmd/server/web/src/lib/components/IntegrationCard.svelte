<script lang="ts">
  import { m } from '$lib/paraglide/messages'
  import Button from '$lib/components/ui/Button.svelte'
  import StatusBadge from '$lib/components/ui/StatusBadge.svelte'
  import GoogleIcon from '$lib/components/ui/GoogleIcon.svelte'
  import { Cross, Trash, X } from '@lucide/svelte'

  interface Props {
    provider: string
    label: string
    description: string
    connected: boolean
    providerEmail?: string
    connectHref: string
    onDisconnect?: () => void
    setupSourceHref?: string
  }
  let {
    provider,
    label,
    description,
    connected,
    providerEmail,
    connectHref,
    onDisconnect,
    setupSourceHref,
  }: Props = $props()

  let confirming = $state(false)
</script>

<div class="rounded-xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-gray-900 p-5 flex items-top gap-5 transition-shadow hover:shadow-sm">
  <!-- Provider icon -->
  <div class="shrink-0 flex items-center justify-center w-11 h-11 rounded-xl bg-zinc-50 dark:bg-zinc-800 border border-zinc-200 dark:border-zinc-700">
    {#if provider === 'google'}
      <GoogleIcon size={8} />
    {:else if provider === 'canva'}
      <img src="/canva.logo.svg" width="30" alt="canva logo" />
    {:else}
      <span class="text-zinc-400 text-sm font-medium">{label[0]}</span>
    {/if}
  </div>

  <!-- Label + description -->
  <div class="flex-1 min-w-0">
    <div class="flex items-center gap-2 flex-wrap">
      <p class="font-semibold text-zinc-900 dark:text-zinc-100 text-sm">{label}</p>
      {#if connected}
        <StatusBadge status="healthy" text={m.integrations_connected_as({ email: providerEmail ?? label })} />
      {:else}
        <StatusBadge status="disabled" text={m.integrations_not_connected()} />
      {/if}
    </div>
    <p class="text-sm text-zinc-500 dark:text-zinc-400 mt-1.5 truncate">{description}</p>

    {#if connected && (setupSourceHref || onDisconnect)}
      <div class="mt-2 flex flex-wrap items-center gap-3">
        {#if !confirming}
          <button
            onclick={() => (confirming = true)}
            class="flex items-center gap-1.5 text-sm text-red-500 hover:underline"
          ><X class="h-3 w-3" /> {m.integrations_disconnect()}</button>
        {:else}
          <span class="text-sm text-zinc-500 dark:text-zinc-400">
            {m.integrations_disconnect_confirm({ provider: label })}
          </span>
          <button
            onclick={() => { confirming = false; onDisconnect?.() }}
            class="text-sm font-semibold text-red-600 hover:underline"
          >{m.integrations_disconnect()}</button>
          <button
            onclick={() => (confirming = false)}
            class="text-sm text-zinc-500 hover:underline"
          >Cancel</button>
        {/if}
      </div>
    {/if}
  </div>

  <!-- Action button -->
  <div class="shrink-0">
    {#if connected}
      {#if setupSourceHref}
        <a href={setupSourceHref}>
          <Button variant="secondary" size="sm">{m.integrations_setup_source().replace(' →', '')}</Button>
        </a>
      {/if}
    {:else}
      <a href={connectHref}>
        <Button variant="outline" size="sm">{m.integrations_connect({ provider: label })}</Button>
      </a>
    {/if}
  </div>
</div>
