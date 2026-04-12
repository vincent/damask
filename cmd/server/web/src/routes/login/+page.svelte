<script lang="ts">
  import { goto } from '$app/navigation'
  import { authApi, configApi, ApiError } from '$lib/api'
  import Button from '$lib/components/ui/Button.svelte'
  import Feedback from '$lib/components/ui/Feedback.svelte'
  import Hint from '$lib/components/ui/Hint.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import Title from '$lib/components/ui/Title.svelte'

  let email = $state('')
  let password = $state('')
  let error = $state('')
  let loading = $state(false)
  let demoLoading = $state(false)
  let isDemo = $state(false)

  $effect(() => {
    configApi.get().then(cfg => { isDemo = cfg.demo }).catch(() => {})
  })

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault()
    error = ''
    loading = true

    try {
      await authApi.login(email, password)
      goto('/library')
    } catch (err) {
      error = err instanceof ApiError ? err.message : 'Login failed'
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
      error = err instanceof ApiError ? err.message : 'Could not start demo session'
    } finally {
      demoLoading = false
    }
  }
</script>

<svelte:head>
  <title>Sign in — Damask</title>
</svelte:head>

<div class="damask-texture-strong relative min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-950">
  <div class="z-1 w-full max-w-md space-y-8 p-8 bg-white dark:bg-gray-900 rounded-xl shadow">
    <div>
      <Title>Sign in</Title>
      <Hint>
        Don't have an account? <a href="/register" class="text-blue-600 hover:underline">Register</a>
      </Hint>
    </div>

    <form onsubmit={handleSubmit} class="space-y-4">
      <Feedback {error} />
      <Input id="email" type="email" label="Email" bind:value={email} required autocomplete="email" />
      <Input id="password" type="password" label="Password" bind:value={password} required autocomplete="current-password" />
      <Button type="submit" {loading} class="w-full">{loading ? 'Signing in…' : 'Sign in'}</Button>
    </form>

    {#if isDemo}
      <div class="text-center">
        <button onclick={handleDemo} disabled={demoLoading} class="text-md text-blue-600 hover:underline disabled:opacity-50">
          {demoLoading ? 'Starting demo…' : 'Try the demo'}
        </button>
      </div>
    {/if}
  </div>
</div>
