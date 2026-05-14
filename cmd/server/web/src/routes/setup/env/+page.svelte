<script lang="ts">
  import { goto } from '$app/navigation'
  import { onDestroy } from 'svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import SetupStep from '$lib/components/SetupStep.svelte'
  import { m } from '$lib/paraglide/messages'
  import {
    defaultEnvParams,
    defaultStorageParams,
    wizardStore,
  } from '$lib/stores/setupWizard'
  import type { EnvParams } from '$lib/api'
  import { setupApi } from '$lib/api/setup'

  let form = $state<EnvParams>(defaultEnvParams())
  let error = $state('')
  let loading = $state(false)
  let smtpOpen = $state(false)
  let oidcOpen = $state(false)

  const unsubscribe = wizardStore.subscribe((state) => {
    const storage = state.storage ?? defaultStorageParams()
    form = state.env ?? { ...defaultEnvParams(), ...storage }
  })

  onDestroy(unsubscribe)

  function patch<K extends keyof EnvParams>(key: K, value: EnvParams[K]) {
    form = { ...form, [key]: value }
    error = ''
  }

  async function onNext() {
    loading = true
    error = ''
    try {
      await setupApi.writeConfig(form)
      wizardStore.update((state) => ({ ...state, env: form }))
      await goto('/setup/owner')
    } catch (err) {
      error = err instanceof Error ? err.message : m.try_again()
    } finally {
      loading = false
    }
  }
</script>

<SetupStep
  title={m.setup_env_title()}
  backHref="/setup/deps"
  {loading}
  {onNext}
>
  <div class="grid gap-4 md:grid-cols-2">
    <Input
      label={m.setup_env_port()}
      type="number"
      value={String(form.port)}
      {error}
      oninput={(e) =>
        patch('port', Number((e.currentTarget as HTMLInputElement).value) || 0)}
    />
    <Input
      label={m.setup_env_base_url()}
      value={form.baseURL}
      oninput={(e) =>
        patch('baseURL', (e.currentTarget as HTMLInputElement).value)}
    />
  </div>
  <p class="text-sm text-slate-500 dark:text-slate-400">
    {m.setup_env_base_url_hint()}
  </p>

  <div
    class="rounded-2xl border border-slate-200 bg-white/80 p-4 dark:border-slate-800 dark:bg-slate-950/60"
  >
    <button
      class="w-full text-left font-medium text-slate-900 dark:text-slate-100"
      onclick={() => (smtpOpen = !smtpOpen)}
    >
      {m.setup_env_smtp()}
    </button>
    {#if smtpOpen}
      <div class="mt-4 grid gap-4 md:grid-cols-2">
        <Input
          label="Host"
          value={form.smtpHost}
          oninput={(e) =>
            patch('smtpHost', (e.currentTarget as HTMLInputElement).value)}
        />
        <Input
          label="Port"
          type="number"
          value={String(form.smtpPort)}
          oninput={(e) =>
            patch(
              'smtpPort',
              Number((e.currentTarget as HTMLInputElement).value) || 0
            )}
        />
        <Input
          label="User"
          value={form.smtpUser}
          oninput={(e) =>
            patch('smtpUser', (e.currentTarget as HTMLInputElement).value)}
        />
        <Input
          label="Password"
          type="password"
          value={form.smtpPass}
          oninput={(e) =>
            patch('smtpPass', (e.currentTarget as HTMLInputElement).value)}
        />
      </div>
    {/if}
  </div>

  <div
    class="rounded-2xl border border-slate-200 bg-white/80 p-4 dark:border-slate-800 dark:bg-slate-950/60"
  >
    <button
      class="w-full text-left font-medium text-slate-900 dark:text-slate-100"
      onclick={() => (oidcOpen = !oidcOpen)}
    >
      {m.setup_env_oidc()}
    </button>
    {#if oidcOpen}
      <div class="mt-4 grid gap-4 md:grid-cols-2">
        <Input
          label="Issuer URL"
          value={form.oidcIssuer}
          oninput={(e) =>
            patch('oidcIssuer', (e.currentTarget as HTMLInputElement).value)}
        />
        <Input
          label="Client ID"
          value={form.oidcClientID}
          oninput={(e) =>
            patch('oidcClientID', (e.currentTarget as HTMLInputElement).value)}
        />
        <Input
          label="Client secret"
          type="password"
          value={form.oidcClientSecret}
          oninput={(e) =>
            patch(
              'oidcClientSecret',
              (e.currentTarget as HTMLInputElement).value
            )}
        />
      </div>
    {/if}
  </div>
</SetupStep>
