<script lang="ts">
  import { m } from '$lib/paraglide/messages'

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

<div class="rounded-xl border border-zinc-200 dark:border-zinc-700 bg-white dark:bg-zinc-900 p-5">
  <div class="flex items-start justify-between gap-4">
    <div class="min-w-0">
      <p class="font-semibold text-zinc-900 dark:text-zinc-100">{label}</p>
      <p class="text-sm text-zinc-500 dark:text-zinc-400 mt-0.5">{description}</p>
    </div>
    <div class="flex items-center gap-1.5 shrink-0 mt-0.5">
      {#if connected}
        <span class="inline-block w-2 h-2 rounded-full bg-green-500"></span>
        <span class="text-sm text-zinc-600 dark:text-zinc-300">
          {providerEmail ? m.integrations_connected_as({ email: providerEmail }) : label}
        </span>
      {:else}
        <span class="inline-block w-2 h-2 rounded-full bg-zinc-300 dark:bg-zinc-600"></span>
        <span class="text-sm text-zinc-500 dark:text-zinc-400">{m.integrations_not_connected()}</span>
      {/if}
    </div>
  </div>

  <div class="mt-4 flex flex-wrap gap-2">
    {#if connected}
      {#if setupSourceHref}
        <a
          href={setupSourceHref}
          class="text-sm text-blue-600 dark:text-blue-400 hover:underline"
        >{m.integrations_setup_source()}</a>
      {/if}
      {#if !confirming}
        <button
          onclick={() => (confirming = true)}
          class="text-sm text-red-500 hover:underline"
        >{m.integrations_disconnect()}</button>
      {:else}
        <span class="text-sm text-zinc-600 dark:text-zinc-300">
          {m.integrations_disconnect_confirm({ provider: label })}
        </span>
        <button
          onclick={() => { confirming = false; onDisconnect?.() }}
          class="text-sm text-red-600 font-medium hover:underline"
        >{m.integrations_disconnect()}</button>
        <button
          onclick={() => (confirming = false)}
          class="text-sm text-zinc-500 hover:underline"
        >Cancel</button>
      {/if}
    {:else}
      <a
        href={connectHref}
        class="inline-flex items-center gap-1.5 text-sm font-medium text-blue-600 dark:text-blue-400 hover:underline"
      >{m.integrations_connect({ provider: label })}</a>
    {/if}
  </div>
</div>
