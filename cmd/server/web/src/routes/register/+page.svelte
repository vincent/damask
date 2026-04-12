<script lang="ts">
  import { goto } from '$app/navigation'
  import { authApi, ApiError } from '$lib/api'
  import Button from '$lib/components/ui/Button.svelte'
  import Feedback from '$lib/components/ui/Feedback.svelte'
  import Hint from '$lib/components/ui/Hint.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import Title from '$lib/components/ui/Title.svelte'

  let name = $state('')
  let email = $state('')
  let password = $state('')
  let error = $state('')
  let loading = $state(false)

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault()
    error = ''
    loading = true

    try {
      await authApi.register(name, email, password)
      goto('/library')
    } catch (err) {
      error = err instanceof ApiError ? err.message : 'Registration failed'
    } finally {
      loading = false
    }
  }
</script>

<svelte:head>
  <title>Create account — Damask</title>
</svelte:head>

<div class="damask-texture-strong relative min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-950">
  <div class="z-1 w-full max-w-md space-y-8 p-8 bg-white dark:bg-gray-900 rounded-xl shadow">
    <div>
      <Title>Create your account</Title>
      <Hint>
        Already have an account? <a href="/login" class="text-blue-600 hover:underline">Sign in</a>
      </Hint>
    </div>

    <form onsubmit={handleSubmit} class="space-y-4">
      <Feedback {error} />
      <Input id="name" type="text" label="Full name" bind:value={name} required autocomplete="name" />
      <Input id="email" type="email" label="Email" bind:value={email} required autocomplete="email" />
      <div>
        <Input id="password" type="password" label="Password" bind:value={password} required autocomplete="new-password" />
        <p class="mt-1 text-sm text-gray-500">Minimum 8 characters</p>
      </div>
      <Button type="submit" {loading} class="w-full">{loading ? 'Creating account…' : 'Create account'}</Button>
    </form>
  </div>
</div>
