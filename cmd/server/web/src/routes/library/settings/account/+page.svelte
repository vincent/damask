<script lang="ts">
  import { authApi, type MeResponse, ApiError } from '$lib/api'
  import LinkedIdentityRow from '$lib/components/LinkedIdentityRow.svelte'
  import PageHeader from '$lib/components/ui/PageHeader.svelte'
  import { m } from '$lib/paraglide/messages'

  let me = $state<MeResponse | null>(null)
  let loading = $state(true)
  let errors = $state<Record<string, string>>({})

  $effect(() => {
    authApi
      .me()
      .then((data) => {
        me = data
      })
      .catch(() => {})
      .finally(() => {
        loading = false
      })
  })

  async function unlink(provider: 'oidc' | 'google' | 'canva') {
    errors = { ...errors, [provider]: '' }
    try {
      if (provider === 'oidc') await authApi.unlinkOIDC()
      else if (provider === 'google') await authApi.unlinkGoogle()
      else await authApi.unlinkCanva()
      me = await authApi.me()
    } catch (err) {
      const msg =
        err instanceof ApiError
          ? err.message
          : m.settings_auth_last_method_error()
      errors = { ...errors, [provider]: msg }
    }
  }
</script>

<svelte:head>
  <title>{m.settings_auth_title()} — Damask</title>
</svelte:head>

<div class="flex-1 overflow-y-auto">
  <PageHeader title={m.settings_auth_title()} />
  <div class="mx-auto w-full max-w-3xl space-y-8 px-8 py-10">
    {#if loading}
      <div class="text-sm text-[var(--text-muted)]">Loading…</div>
    {:else if me}
      <div
        class="divide-y divide-[var(--border-subtle)] rounded-xl border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-5"
      >
        <LinkedIdentityRow
          provider="password"
          label={m.settings_auth_password()}
          linked={me.auth_methods.includes('password')}
        />

        <LinkedIdentityRow
          provider="google"
          label="Google"
          linked={me.google_linked}
          connectHref="/auth/google/login"
          disconnectError={errors.google}
          onDisconnect={() => unlink('google')}
        />

        <LinkedIdentityRow
          provider="oidc"
          label="SSO"
          linked={me.oidc_linked}
          connectHref="/auth/oidc/login"
          disconnectError={errors.oidc}
          onDisconnect={() => unlink('oidc')}
        />

        <LinkedIdentityRow
          provider="canva"
          label="Canva"
          linked={me.canva_linked}
          connectHref="/auth/canva/login"
          disconnectError={errors.canva}
          onDisconnect={() => unlink('canva')}
        />
      </div>
    {/if}
  </div>
</div>
