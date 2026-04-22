<script lang="ts">
  import { integrationsApi, type OAuthConnection } from '$lib/api'
  import IntegrationCard from '$lib/components/IntegrationCard.svelte'
  import { m } from '$lib/paraglide/messages'

  let connections = $state<OAuthConnection[]>([])
  let loading = $state(true)

  $effect(() => {
    integrationsApi.list()
      .then(c => { connections = c })
      .catch(() => {})
      .finally(() => { loading = false })
  })

  function connectionFor(provider: string): OAuthConnection | undefined {
    return connections.find(c => c.provider === provider)
  }

  async function disconnect(id: string) {
    await integrationsApi.disconnect(id)
    connections = connections.filter(c => c.id !== id)
  }
</script>

<svelte:head>
  <title>{m.integrations_title()} — Damask</title>
</svelte:head>

<div class="max-w-2xl mx-auto py-10 px-4 space-y-6">
  <div>
    <h1 class="text-2xl font-bold text-zinc-900 dark:text-zinc-100">{m.integrations_title()}</h1>
    <p class="mt-1 text-sm text-zinc-500 dark:text-zinc-400">{m.integrations_subtitle()}</p>
  </div>

  {#if loading}
    <div class="text-sm text-zinc-400 dark:text-zinc-500">Loading…</div>
  {:else}
    {@const googleConn = connectionFor('google')}
    <IntegrationCard
      provider="google"
      label={m.integrations_google_drive_label()}
      description={m.integrations_google_drive_desc()}
      connected={!!googleConn}
      providerEmail={googleConn?.provider_email}
      connectHref="/integrations/connect/google"
      setupSourceHref={googleConn ? `/settings/ingress/new?connection=${googleConn.id}&type=gdrive` : undefined}
      onDisconnect={googleConn ? () => disconnect(googleConn.id) : undefined}
    />

    {@const canvaConn = connectionFor('canva')}
    <IntegrationCard
      provider="canva"
      label={m.integrations_canva_label()}
      description={m.integrations_canva_desc()}
      connected={!!canvaConn}
      providerEmail={canvaConn?.provider_email}
      connectHref="/integrations/connect/canva"
      setupSourceHref={canvaConn ? `/settings/ingress/new?connection=${canvaConn.id}&type=canva` : undefined}
      onDisconnect={canvaConn ? () => disconnect(canvaConn.id) : undefined}
    />
  {/if}
</div>
