<script lang="ts">
  import { goto } from '$app/navigation'
  import { page } from '$app/stores'
  import { workspaceApi, ApiError } from '$lib/api'
  import Button from '$lib/components/ui/Button.svelte'
  import Feedback from '$lib/components/ui/Feedback.svelte'
  import Hint from '$lib/components/ui/Hint.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import Title from '$lib/components/ui/Title.svelte'
  import { m } from '$lib/paraglide/messages'

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
      error = err instanceof ApiError ? err.message : m.cannot_accept_invite()
    } finally {
      loading = false
    }
  }
</script>

<svelte:head>
  <title>{m.accept_invite()} — Damask</title>
</svelte:head>

<div
  class="damask-texture-strong relative flex min-h-screen items-center justify-center bg-gray-50 dark:bg-gray-950"
>
  <div
    class="z-1 w-full max-w-md space-y-8 rounded-xl bg-white p-8 shadow dark:bg-gray-900"
  >
    {#if token.trim() === ''}
      <div>
        <Title>{m.invalid_invite()}</Title>
        <Hint>{m.invite_missing_token()}</Hint>
      </div>
    {:else}
      <div>
        <Title>{m.accept_invite()}</Title>
        <Hint>{m.create_account_join_workspace()}</Hint>
      </div>

      <form onsubmit={handleSubmit} class="space-y-4">
        <Feedback {error} />
        <Input
          id="name"
          type="text"
          label={m.name()}
          bind:value={name}
          required
          autocomplete="name"
        />
        <Input
          id="password"
          type="password"
          label={m.password()}
          bind:value={password}
          required
          autocomplete="new-password"
          placeholder={m.min_8_chars()}
        />
        <Button type="submit" {loading} class="w-full"
          >{loading ? m.joining() : m.join_workspace()}</Button
        >
      </form>
    {/if}
  </div>
</div>
