<script lang="ts">
  import { onMount } from 'svelte'
  import { goto } from '$app/navigation'
  import { authStore } from '$lib/stores/auth'

  onMount(async () => {
    // Clear cookie by setting expired date via a server action
    await fetch('http://localhost:8080/auth/logout', {
      method: 'POST',
      credentials: 'include',
    }).catch(() => {})
    authStore.logout()
    goto('/login')
  })
</script>

<p>Signing out…</p>
