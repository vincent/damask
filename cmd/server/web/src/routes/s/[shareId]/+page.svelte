<script lang="ts">
  import { onMount } from 'svelte'
  import { goto } from '$app/navigation'
  import { page } from '$app/state'
  import { Lock } from '@lucide/svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import PoweredByDamask from '$lib/components/PoweredByDamask.svelte'
  import { m } from '$lib/paraglide/messages'

  const API_BASE = import.meta.env.VITE_API_URL ?? ''

  let shareId = $derived(page.params.shareId)
  let password = $state('')
  let error = $state('')
  let loading = $state(false)
  let checking = $state(true)
  let shareLabel = $state(m.shared_gallery())

  onMount(async () => {
    // Check if we already have a session token
    const existing = (await cookieStore.get(`share_token_${shareId}`))?.value || null
    if (existing) {
      goto(`/s/${shareId}/view`, { replaceState: true })
      return
    }

    // Probe share metadata to determine if a password is required
    try {
      const res = await fetch(`${API_BASE}/shared/${shareId}/access`)
      if (res.ok) {
        const data = await res.json()
        shareLabel = data.label ?? m.shared_gallery()
        if (!data.has_password) {
          // No password required — obtain a token and proceed directly to view
          await acquireToken('')
          return
        }
        // Password required — fall through to show the password form
      } else if (res.status === 404 || res.status === 410) {
        error = m.link_expired()
      } else {
        error = m.load_shared_failed()
      }
    } catch {
      error = m.load_page_failed()
    }
    checking = false
  })

  async function acquireToken(pwd: string): Promise<boolean> {
    const res = await fetch(`${API_BASE}/shared/${shareId}/access`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password: pwd }),
    })
    if (!res.ok) return false
    const data = await res.json()
    await cookieStore.set(`share_token_${shareId}`, data.token)
    goto(`/s/${shareId}/view`, { replaceState: true })
    return true
  }

  async function handleAccess() {
    if (!password.trim()) {
      error = m.enter_password()
      return
    }
    loading = true
    error = ''
    try {
      const ok = await acquireToken(password)
      if (!ok) error = m.incorrect_password()
    } catch {
      error = m.try_again()
    }
    loading = false
  }
</script>

<svelte:head>
  <title>{shareLabel} — Damask</title>
</svelte:head>

<div class="flex min-h-screen flex-col items-center justify-center bg-gray-50 dark:bg-gray-950 px-4">
  {#if checking}
    <div class="flex items-center gap-3 text-gray-500 dark:text-gray-400">
      <svg class="h-5 w-5 animate-spin" viewBox="0 0 24 24" fill="none">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
      </svg>
      <span class="text-md">{m.loading()}</span>
    </div>
  {:else if error && !password}
    <div class="w-full max-w-md rounded-2xl bg-white dark:bg-gray-900 p-8 shadow-sm text-center">
      <p class="text-gray-600 dark:text-gray-400 text-md">{error}</p>
    </div>
  {:else}
    <div class="w-full max-w-md rounded-2xl bg-white dark:bg-gray-900 p-8 shadow-sm">
      <!-- Lock icon -->
      <div class="mb-5 flex justify-center">
        <div class="flex h-12 w-12 items-center justify-center rounded-xl bg-indigo-100 dark:bg-indigo-900/50">
          <Lock class="h-6 w-6 text-indigo-600 dark:text-indigo-400" />
        </div>
      </div>

      <h1 class="mb-2 text-center text-2xl font-bold text-gray-900 dark:text-gray-100">
        {shareLabel}
      </h1>
      <p class="mb-6 text-center text-md text-gray-500 dark:text-gray-400">
        {m.gallery_is_password_protected()}
      </p>

      <form onsubmit={(e) => { e.preventDefault(); handleAccess() }}>
        <Input
          label={m.password()}
          type="password"
          placeholder={m.password()}
          bind:value={password}
          {error}
          autofocus
        />

        <Button
          type="submit"
          variant="primary"
          {loading}
          class="mt-4 w-full"
        >
          {#snippet icon()}
            {#if !loading}
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M5 12h14M12 5l7 7-7 7" stroke-linecap="round" stroke-linejoin="round" />
              </svg>
            {/if}
          {/snippet}
          {m.access_gallery()}
        </Button>
      </form>
    </div>
  {/if}

  <!-- Footer -->
  <PoweredByDamask />
</div>
