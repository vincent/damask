<script lang="ts">
  import { goto } from '$app/navigation'
  import { onMount } from 'svelte'
  import type { HealthResponse } from '$lib/api'
  import { apiFetch } from '$lib/api/client'
  import { m } from '$lib/paraglide/messages'

  let version = $state('dev')

  onMount(() => {
    if (typeof __APP_VERSION__ !== 'undefined') {
      version = __APP_VERSION__
    }

    const timeout = setTimeout(async () => {
      try {
        const health = await apiFetch<HealthResponse>('/health')
        if (!health.setupRequired) {
          await goto('/library')
          return
        }
      } catch {
        // Keep routing to the wizard on server startup churn.
      }
      await goto('/setup/storage')
    }, 1500)

    return () => clearTimeout(timeout)
  })
</script>

<div class="flex min-h-[60vh] items-center justify-center">
  <div
    class="rounded-[2rem] border border-white/60 bg-white/90 px-10 py-14 text-center shadow-[0_24px_70px_rgba(15,23,42,0.12)] backdrop-blur dark:border-slate-800 dark:bg-slate-900/90"
  >
    <div
      class="mx-auto mb-5 flex h-16 w-16 items-center justify-center rounded-2xl bg-cyan-500 text-3xl font-black text-white shadow-lg shadow-cyan-500/30"
    >
      d
    </div>
    <h1
      class="font-[Bricolage_Grotesque] text-4xl font-bold text-slate-900 dark:text-white"
    >
      Damask
    </h1>
    <p class="mt-3 text-sm text-slate-600 dark:text-slate-300">
      {m.setup_splash_tagline()}
    </p>
    <p
      class="mt-5 text-xs tracking-[0.2em] text-slate-400 uppercase dark:text-slate-500"
    >
      {version}
    </p>
  </div>
</div>
