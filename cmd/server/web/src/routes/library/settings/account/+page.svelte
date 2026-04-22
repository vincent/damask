<script lang="ts">
  import { authApi, type MeResponse, ApiError } from '$lib/api'
  import LinkedIdentityRow from '$lib/components/LinkedIdentityRow.svelte'
  import { m } from '$lib/paraglide/messages'

  let me = $state<MeResponse | null>(null)
  let loading = $state(true)
  let errors = $state<Record<string, string>>({})

  $effect(() => {
    authApi.me()
      .then(data => { me = data })
      .catch(() => {})
      .finally(() => { loading = false })
  })

  async function unlink(provider: 'oidc' | 'google' | 'canva') {
    errors = { ...errors, [provider]: '' }
    try {
      if (provider === 'oidc') await authApi.unlinkOIDC()
      else if (provider === 'google') await authApi.unlinkGoogle()
      else await authApi.unlinkCanva()
      me = await authApi.me()
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : m.settings_auth_last_method_error()
      errors = { ...errors, [provider]: msg }
    }
  }
</script>

<svelte:head>
  <title>{m.settings_auth_title()} — Damask</title>
</svelte:head>

<div class="max-w-2xl mx-auto py-10 px-4 space-y-8">
  <div>
    <h1 class="text-2xl font-bold text-zinc-900 dark:text-zinc-100">{m.settings_auth_title()}</h1>
  </div>

  {#if loading}
    <div class="text-sm text-zinc-400 dark:text-zinc-500">Loading…</div>
  {:else if me}
    <div class="rounded-xl border border-zinc-200 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-5 divide-y divide-zinc-100 dark:divide-zinc-800">
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
