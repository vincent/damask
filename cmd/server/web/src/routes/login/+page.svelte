<script lang="ts">
  import { goto } from '$app/navigation'
  import { page } from '$app/state'
  import { authApi, ApiError } from '$lib/api'
  import { configStore } from '$lib/stores/config.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import Feedback from '$lib/components/ui/Feedback.svelte'
  import Hint from '$lib/components/ui/Hint.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import Title from '$lib/components/ui/Title.svelte'
  import OAuthButton from '$lib/components/OAuthButton.svelte'
  import { m } from '$lib/paraglide/messages'

  interface AuthConfig {
    password_auth: boolean
    signup_enabled: boolean
    oidc_enabled: boolean
    oidc_label: string
    google_enabled: boolean
    canva_enabled: boolean
  }

  let authConfig = $state<AuthConfig>({
    password_auth: true,
    signup_enabled: true,
    oidc_enabled: false,
    oidc_label: 'Sign in with SSO',
    google_enabled: false,
    canva_enabled: false,
  })

  $effect(() => {
    fetch('/config/auth')
      .then((r) => r.json())
      .then((d) => {
        authConfig = d
      })
      .catch(() => {
        authConfig = { ...authConfig, password_auth: true }
      })
  })

  const ssoErrorMessages: Record<string, string> = {
    oidc_error: 'The identity provider returned an error.',
    oidc_exchange: 'Could not complete sign-in. Please try again.',
    email_not_verified:
      'Your email address is not verified with your identity provider.',
  }

  const ssoError = $derived(
    (() => {
      const e = page.url.searchParams.get('error')
      return e ? (ssoErrorMessages[e] ?? m.login_failed()) : ''
    })()
  )

  const resetSuccess = $derived(page.url.searchParams.get('reset') === '1')
  const accountDeleted = $derived(
    page.url.searchParams.get('account_deleted') === '1'
  )

  const hasSSOProviders = $derived(
    authConfig.oidc_enabled ||
      authConfig.google_enabled ||
      authConfig.canva_enabled
  )

  let email = $state('')
  let password = $state('')
  let error = $state('')
  let loading = $state(false)
  let demoLoading = $state(false)

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault()
    error = ''
    loading = true
    try {
      await authApi.login(email, password)
      goto('/library')
    } catch (err) {
      error = err instanceof ApiError ? err.message : m.login_failed()
    } finally {
      loading = false
    }
  }

  async function handleDemo() {
    demoLoading = true
    error = ''
    try {
      await authApi.demoSession()
      goto('/library')
    } catch (err) {
      error = err instanceof ApiError ? err.message : m.cannot_start_demo()
    } finally {
      demoLoading = false
    }
  }
</script>

<svelte:head>
  <title>{m.signin()} — Damask</title>
</svelte:head>

<div
  class="damask-texture-strong relative flex min-h-screen items-center justify-center bg-gray-50 dark:bg-gray-950"
>
  <div
    class="z-1 w-full max-w-md space-y-8 rounded-xl bg-white p-8 shadow dark:bg-gray-900"
  >
    <div>
      <Title>{m.signin()}</Title>
      {#if authConfig.signup_enabled}
        <Hint>
          {m.no_account_question()}
          <a href="/register" class="text-blue-600 hover:underline"
            >{m.register()}</a
          >
        </Hint>
      {/if}
    </div>

    {#if ssoError}
      <Feedback error={ssoError} />
    {/if}

    {#if resetSuccess}
      <Feedback success="Password updated. You can now sign in." />
    {/if}

    {#if accountDeleted}
      <Feedback success="Your account has been deleted." />
    {/if}

    {#if hasSSOProviders}
      <div class="space-y-2">
        {#if authConfig.oidc_enabled}
          <OAuthButton
            provider="oidc"
            label={authConfig.oidc_label}
            href="/auth/oidc/login"
          />
        {/if}
        {#if authConfig.google_enabled}
          <OAuthButton
            provider="google"
            label={m.auth_sso_google()}
            href="/auth/google/login"
          />
        {/if}
        {#if authConfig.canva_enabled}
          <OAuthButton
            provider="canva"
            label={m.auth_sso_canva()}
            href="/auth/canva/login"
          />
        {/if}
      </div>
    {/if}

    {#if hasSSOProviders && authConfig.password_auth}
      <div class="relative flex items-center gap-3">
        <div class="flex-1 border-t border-zinc-200 dark:border-zinc-700"></div>
        <span class="text-xs text-zinc-400 dark:text-zinc-500">or</span>
        <div class="flex-1 border-t border-zinc-200 dark:border-zinc-700"></div>
      </div>
    {/if}

    {#if authConfig.password_auth}
      <form onsubmit={handleSubmit} class="space-y-4">
        <Feedback {error} />
        <Input
          id="email"
          type="email"
          label={m.email()}
          bind:value={email}
          required
          autocomplete="email"
        />
        <Input
          id="password"
          type="password"
          label={m.password()}
          bind:value={password}
          required
          autocomplete="current-password"
        />
        {#if authConfig.password_auth}
          <div class="text-right">
            <a
              href="/forgot-password"
              class="text-sm text-blue-600 hover:underline">Forgot password?</a
            >
          </div>
        {/if}
        <Button type="submit" {loading} class="w-full"
          >{loading ? m.signin_in() : m.signin()}</Button
        >
      </form>
    {/if}

    {#if configStore.state.demo}
      <div class="text-center">
        <button
          onclick={handleDemo}
          disabled={demoLoading}
          class="text-md text-blue-600 hover:underline disabled:opacity-50 dark:text-gray-50"
        >
          {demoLoading ? m.starting_demo() : m.try_demo()}
        </button>
      </div>
    {/if}
  </div>
</div>
