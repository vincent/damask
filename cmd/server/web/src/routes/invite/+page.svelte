<script lang="ts">
  import { goto } from '$app/navigation'
  import { page } from '$app/stores'
  import { workspaceApi, ApiError } from '$lib/api'
  import Button from '$lib/components/ui/Button.svelte'
  import Feedback from '$lib/components/ui/Feedback.svelte'
  import Hint from '$lib/components/ui/Hint.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import Title from '$lib/components/ui/Title.svelte'

  const token = $page.url.searchParams.get('token') ?? ''

  let name = $state('')
  let password = $state('')
  let error = $state('')
  let loading = $state(false)

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault()
    error = ''
    loading = true

    try {
      await workspaceApi.acceptInvite(token, name, password)
      goto('/library')
    } catch (err) {
      error = err instanceof ApiError ? err.message : 'Failed to accept invitation'
    } finally {
      loading = false
    }
  }
</script>

<svelte:head>
  <title>Accept invitation — Damask</title>
</svelte:head>

<div class="damask-texture-strong relative min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-950">
  <div class="z-1 w-full max-w-md space-y-8 p-8 bg-white dark:bg-gray-900 rounded-xl shadow">
    {#if token.trim() === ''}
      <div>
        <Title>Invalid invitation</Title>
        <Hint>This invite link is missing a token. Check that you copied the full URL.</Hint>
      </div>
    {:else}
      <div>
        <Title>Accept invitation</Title>
        <Hint>Create your account to join the workspace.</Hint>
      </div>

      <form onsubmit={handleSubmit} class="space-y-4">
        <Feedback {error} />
        <Input id="name" type="text" label="Name" bind:value={name} required autocomplete="name" />
        <Input id="password" type="password" label="Password" bind:value={password} required autocomplete="new-password" placeholder="minimum 8 characters" />
        <Button type="submit" {loading} class="w-full">{loading ? 'Joining…' : 'Join workspace'}</Button>
      </form>
    {/if}
  </div>
</div>
