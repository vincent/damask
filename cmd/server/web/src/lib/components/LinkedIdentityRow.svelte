<script lang="ts">
  import { m } from '$lib/paraglide/messages'

  interface Props {
    provider: 'password' | 'oidc' | 'google' | 'canva'
    label: string
    linked: boolean
    email?: string
    connectHref?: string
    onDisconnect?: () => void
    disconnectError?: string
  }

  let {
    provider,
    label,
    linked,
    email,
    connectHref,
    onDisconnect,
    disconnectError,
  }: Props = $props()
</script>

<div
  class="flex items-center justify-between gap-4 border-b border-zinc-100 py-4 last:border-0 dark:border-zinc-800"
>
  <div class="min-w-0">
    <p class="font-medium text-zinc-900 dark:text-zinc-100">{label}</p>
    {#if linked && email}
      <p class="text-sm text-zinc-500 dark:text-zinc-400">
        {m.settings_auth_connected_as({ email })}
      </p>
    {:else if linked}
      <p class="text-sm text-zinc-500 dark:text-zinc-400">
        {m.settings_auth_connected_as({ email: label })}
      </p>
    {:else if provider !== 'password'}
      <p class="text-sm text-zinc-400 dark:text-zinc-500">—</p>
    {/if}
    {#if disconnectError}
      <p class="mt-0.5 text-sm text-red-500">{disconnectError}</p>
    {/if}
  </div>

  <div class="shrink-0">
    {#if provider === 'password'}
      <!-- password row: no connect/disconnect, just a placeholder -->
    {:else if linked}
      <button
        type="button"
        onclick={onDisconnect}
        class="text-sm text-red-500 hover:underline"
        >{m.settings_auth_disconnect()}</button
      >
    {:else if connectHref}
      <a
        href={connectHref}
        class="text-sm text-blue-600 hover:underline dark:text-blue-400"
        >{m.settings_auth_connect()}</a
      >
    {/if}
  </div>
</div>
