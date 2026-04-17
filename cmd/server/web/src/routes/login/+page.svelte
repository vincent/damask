<script lang="ts">
  import { goto } from '$app/navigation'
  import { authApi, ApiError } from '$lib/api'
  import { configStore } from '$lib/stores/config.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import Feedback from '$lib/components/ui/Feedback.svelte'
  import Hint from '$lib/components/ui/Hint.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import Title from '$lib/components/ui/Title.svelte'
  import { m } from '$lib/paraglide/messages'

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

<div class="damask-texture-strong relative min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-950">
  <div class="z-1 w-full max-w-md space-y-8 p-8 bg-white dark:bg-gray-900 rounded-xl shadow">
    <div>
      <Title>{m.signin()}</Title>
      <Hint>
        {m.no_account_question()} <a href="/register" class="text-blue-600 hover:underline">{m.register()}</a>
      </Hint>
    </div>

    <form onsubmit={handleSubmit} class="space-y-4">
      <Feedback {error} />
      <Input id="email" type="email" label={m.email()} bind:value={email} required autocomplete="email" />
      <Input id="password" type="password" label={m.password()} bind:value={password} required autocomplete="current-password" />
      <Button type="submit" {loading} class="w-full">{loading ? m.signin_in() : m.signin()}</Button>
    </form>

    {#if configStore.state.demo}
      <div class="text-center">
        <button onclick={handleDemo} disabled={demoLoading} class="text-md text-blue-600 hover:underline disabled:opacity-50">
          {demoLoading ? m.starting_demo() : m.try_demo()}
        </button>
      </div>
    {/if}
  </div>
</div>
