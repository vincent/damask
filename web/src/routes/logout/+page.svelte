<script lang="ts">
  import { onMount } from 'svelte'
  import { goto } from '$app/navigation'
  import { authStore } from '$lib/stores/auth.svelte'
  import { apiFetch } from '$lib/api'

  onMount(async () => {
    await apiFetch('/auth/logout', { method: 'POST' }).catch(() => {})
    authStore.logout()
    goto('/login')
  })
</script>

<p>Signing out…</p>
