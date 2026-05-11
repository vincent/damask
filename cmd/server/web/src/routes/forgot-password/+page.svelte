<script lang="ts">
  import '../auth-page.css'
  import { authApi, ApiError } from '$lib/api'
  import { CircleCheck } from '@lucide/svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import Feedback from '$lib/components/ui/Feedback.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import GeometricBackground from '$lib/components/ui/GeometricBackground.svelte'
  import Divider from '$lib/components/ui/Divider.svelte'

  let email = $state('')
  let loading = $state(false)
  let sent = $state(false)
  let error = $state('')

  async function submit(event: SubmitEvent) {
    event.preventDefault()
    loading = true
    error = ''
    try {
      await authApi.forgotPassword(email)
      sent = true
    } catch (err) {
      error =
        err instanceof ApiError ? err.message : 'Could not send reset link.'
    } finally {
      loading = false
    }
  }
</script>

<svelte:head>
  <title>Reset your password — Damask</title>
</svelte:head>

<div class="auth-root">
  <GeometricBackground />

  <div class="auth-content w-full md:max-w-lg">
    <div class="auth-card">
      <Divider />

      <h1 class="auth-heading">Reset your password</h1>

      {#if sent}
        <div class="fp-sent">
          <CircleCheck size={16} class="fp-sent-icon" />
          <p>Check your email for a reset link.</p>
        </div>
        <p class="auth-body">
          If an account exists for <strong>{email}</strong>, you'll receive
          instructions shortly.
        </p>
        <a href="/login" class="fp-back-link">Back to sign in</a>
      {:else}
        <p class="auth-body">
          Enter your email and we'll send you a reset link.
        </p>
        <form onsubmit={submit} class="fp-form">
          <Feedback {error} class="mx-0" />
          <Input
            label="Email"
            type="email"
            bind:value={email}
            required
            autocomplete="email"
          />
          <Button type="submit" {loading} class="w-full">Send reset link</Button
          >
        </form>
        <a href="/login" class="fp-back-link">Back to sign in</a>
      {/if}
    </div>

    <p class="auth-footer">Damask DAM</p>
  </div>
</div>

<style>
  .fp-form {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    margin-bottom: 1.25rem;
  }

  /* Sent confirmation */
  .fp-sent {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-bottom: 0.75rem;
    color: #b8936a;
    font-size: 0.8125rem;
    font-weight: 500;
  }

  :global(.fp-sent-icon) {
    flex-shrink: 0;
    color: #b8936a;
  }

  .fp-back-link {
    display: block;
    text-align: center;
    font-size: 0.75rem;
    color: #4a3a2a;
    text-decoration: none;
    transition: color 0.15s ease;
  }
  .fp-back-link:hover {
    color: #b8936a;
  }
  .fp-back-link:focus-visible {
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
    .fp-back-link {
      transition: color 0.15s ease;
    }
  }
</style>
