<script lang="ts">
  import '../auth-page.css'
  import { goto } from '$app/navigation'
  import { page } from '$app/state'
  import { authApi, ApiError } from '$lib/api'
  import Button from '$lib/components/ui/Button.svelte'
  import Feedback from '$lib/components/ui/Feedback.svelte'
  import GeometricBackground from '$lib/components/ui/GeometricBackground.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import { onMount } from 'svelte'
  import Divider from '$lib/components/ui/Divider.svelte'

  let password = $state('')
  let confirm = $state('')
  let loading = $state(false)
  let error = $state('')

  const token = $derived(page.url.searchParams.get('token') ?? '')

  onMount(() => {
    // if (!token) goto('/forgot-password')
  })

  async function submit(event: SubmitEvent) {
    event.preventDefault()
    if (password !== confirm) {
      error = 'Passwords do not match.'
      return
    }
    loading = true
    error = ''
    try {
      await authApi.resetPassword(token, password)
      goto('/login?reset=1')
    } catch (err) {
      error =
        err instanceof ApiError && err.status === 400
          ? 'This reset link is invalid or has expired.'
          : err instanceof ApiError
            ? err.message
            : 'Could not reset password.'
    } finally {
      loading = false
    }
  }
</script>

<svelte:head>
  <title>Choose a new password — Damask</title>
</svelte:head>

<div class="auth-root">
  <GeometricBackground />

  <div class="auth-content w-full md:max-w-lg">
    <div class="auth-card">
      <Divider />

      <h1 class="auth-heading">Choose a new password</h1>
      <p class="auth-body">
        Pick something strong — you won't need to change it again.
      </p>

      <form onsubmit={submit} class="rp-form">
        <Feedback {error} class="mx-0" />
        <Input
          label="New password"
          type="password"
          bind:value={password}
          required
          autocomplete="new-password"
        />
        <Input
          label="Confirm new password"
          type="password"
          bind:value={confirm}
          required
          autocomplete="new-password"
        />
        <Button type="submit" {loading} class="w-full">Set new password</Button>
      </form>

      <a href="/login" class="rp-back-link">Back to sign in</a>
    </div>

    <p class="auth-footer">Damask DAM</p>
  </div>
</div>

<style>
  .rp-form {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    margin-bottom: 1.25rem;
  }

  .rp-back-link {
    display: block;
    text-align: center;
    font-size: 0.75rem;
    color: #4a3a2a;
    text-decoration: none;
    transition: color 0.15s ease;
  }
  .rp-back-link:hover {
    color: #b8936a;
  }
  .rp-back-link:focus-visible {
    outline: 2px solid #b8936a;
    outline-offset: 2px;
    border-radius: 2px;
  }

  /* Input + button palette overrides for the dark card */
  :global(.auth-card label) {
    color: #7a6a55;
    font-size: 0.6875rem;
    font-weight: 500;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }

  :global(
    .auth-card input[type='email'],
    .auth-card input[type='password'],
    .auth-card input[type='text']
  ) {
    background: #1a1730;
    border-color: #2a2240;
    color: #e8dcc8;
    caret-color: #b8936a;
  }
  :global(.auth-card input::placeholder) {
    color: #4a3a2a;
  }
  :global(.auth-card input:focus) {
    border-color: #b8936a55;
    box-shadow: 0 0 0 2px #b8936a22;
    outline: none;
  }

  :global(.auth-card button[type='submit']) {
    background: #b8936a;
    color: #0f0c08;
    border: none;
  }
  :global(.auth-card button[type='submit']:hover:not(:disabled)) {
    background: #c9a87c;
  }
  :global(.auth-card button[type='submit']:focus-visible) {
    outline: 2px solid #b8936a;
    outline-offset: 2px;
  }

  @media (prefers-reduced-motion: no-preference) {
    .rp-back-link {
      transition: color 0.15s ease;
    }
  }
</style>
