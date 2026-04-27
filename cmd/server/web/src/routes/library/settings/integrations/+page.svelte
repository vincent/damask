<script lang="ts">
  import { integrationsApi, type OAuthConnection } from '$lib/api'
  import IntegrationCard from '$lib/components/IntegrationCard.svelte'
  import PageHeader from '$lib/components/ui/PageHeader.svelte'
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

<div class="flex-1 overflow-y-auto">
  <PageHeader title={m.integrations_title()} description={m.integrations_subtitle()} />

  <div class="mx-auto w-full max-w-4xl py-8">
    {#if loading}
      <div class="space-y-3">
        {#each [0, 1] as _}
          <div class="rounded-xl border border-zinc-200 dark:border-zinc-700 bg-white dark:bg-zinc-900 p-5 flex items-center gap-5 animate-pulse">
            <div class="w-11 h-11 rounded-xl bg-zinc-100 dark:bg-zinc-800 shrink-0"></div>
            <div class="flex-1 space-y-2">
              <div class="h-3.5 w-32 rounded bg-zinc-100 dark:bg-zinc-800"></div>
              <div class="h-3 w-56 rounded bg-zinc-100 dark:bg-zinc-800"></div>
            </div>
            <div class="h-8 w-20 rounded-lg bg-zinc-100 dark:bg-zinc-800 shrink-0"></div>
          </div>
        {/each}
      </div>
    {:else}
      <div class="space-y-3">
        {#if connectionFor('google') !== undefined}
          {@const googleConn = connectionFor('google')!}
          <IntegrationCard
            provider="google"
            label={m.integrations_google_drive_label()}
            description={m.integrations_google_drive_desc()}
            connected={true}
            providerEmail={googleConn.provider_email}
            connectHref="/integrations/connect/google"
            setupSourceHref={`/settings/ingress/new?connection=${googleConn.id}&type=gdrive`}
            onDisconnect={() => disconnect(googleConn.id)}
          />
        {:else}
          <IntegrationCard
            provider="google"
            label={m.integrations_google_drive_label()}
            description={m.integrations_google_drive_desc()}
            connected={false}
            connectHref="/integrations/connect/google"
          />
        {/if}

        {#if connectionFor('canva') !== undefined}
          {@const canvaConn = connectionFor('canva')!}
          <IntegrationCard
            provider="canva"
            label={m.integrations_canva_label()}
            description={m.integrations_canva_desc()}
            connected={true}
            providerEmail={canvaConn.provider_email}
            connectHref="/integrations/connect/canva"
            setupSourceHref={`/settings/ingress/new?connection=${canvaConn.id}&type=canva`}
            onDisconnect={() => disconnect(canvaConn.id)}
          />
        {:else}
          <IntegrationCard
            provider="canva"
            label={m.integrations_canva_label()}
            description={m.integrations_canva_desc()}
            connected={false}
            connectHref="/integrations/connect/canva"
          />
        {/if}
      </div>
    {/if}
  </div>
</div>
