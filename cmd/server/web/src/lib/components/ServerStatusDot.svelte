<script lang="ts">
  import { LoaderCircle, RotateCw, Wrench } from '@lucide/svelte'
  import { goto } from '$app/navigation'
  import { m } from '$lib/paraglide/messages'
  import { reconnect, serverStatus } from '$lib/stores/serverStatus.svelte'

  let reconnecting = $state(false)

  const statusText = $derived.by(() => {
    const status = $serverStatus
    if (status.setupRequired) return m.server_status_setup_required()
    if (status.state === 'ok') {
      return m.server_status_ok({ latency: String(status.latencyMs ?? 0) })
    }
    if (status.state === 'degraded') {
      return m.server_status_degraded({
        latency: String(status.latencyMs ?? 0),
      })
    }
    if (status.state === 'offline') return m.server_status_offline()
    return m.server_status_connecting()
  })

  async function handleClick() {
    if ($serverStatus.setupRequired) {
      await goto('/setup')
      return
    }
    if ($serverStatus.state !== 'offline') return
    reconnecting = true
    reconnect()
    setTimeout(() => {
      reconnecting = false
    }, 1000)
  }
</script>

<button
  class="inline-flex items-center gap-2 px-3 py-2 text-sm font-medium transition"
  title={statusText}
  onclick={handleClick}
>
  {#if reconnecting}
    <RotateCw class="h-3.5 w-3.5 animate-spin" />
  {:else if $serverStatus.setupRequired}
    <Wrench class="h-3.5 w-3.5 text-sky-500" />
  {:else if $serverStatus.state === 'connecting'}
    <LoaderCircle class="h-3.5 w-3.5 animate-spin text-amber-500" />
  {:else}
    <span class="status-dot h-3.5 w-3.5 {$serverStatus.state}"></span>
  {/if}
  <span class="text-[var(--text-muted)]">{statusText}</span>
</button>

<style>
  .status-dot {
    border-radius: 9999px;
    display: inline-block;
  }

  .status-dot.ok {
    background: rgb(34 197 94);
  }

  .status-dot.degraded,
  .status-dot.connecting {
    background: rgb(245 158 11);
  }

  .status-dot.offline {
    background: rgb(239 68 68);
  }

  @media (prefers-reduced-motion: no-preference) {
    .status-dot.degraded {
      animation: pulse 1.6s infinite;
    }
  }

  @keyframes pulse {
    0%,
    100% {
      opacity: 1;
    }
    50% {
      opacity: 0.45;
    }
  }
</style>
