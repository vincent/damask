<script lang="ts">
  import { page } from '$app/state'
  import { m } from '$lib/paraglide/messages'

  let { children }: { children?: any } = $props()

  const steps = [
    { href: '/setup', label: m.setup_step_done() },
    { href: '/setup/storage', label: m.setup_step_storage() },
    { href: '/setup/deps', label: m.setup_step_deps() },
    { href: '/setup/env', label: m.setup_step_env() },
    { href: '/setup/owner', label: m.setup_step_owner() },
  ]

  const currentIndex = $derived.by(() => {
    const pathname = page.url.pathname
    const found = steps.findIndex((step) => step.href === pathname)
    return found === -1 ? 0 : found
  })

  const progress = $derived(
    Math.max(8, ((currentIndex + 1) / steps.length) * 100)
  )
</script>

<svelte:head>
  <title>Setup — Damask</title>
</svelte:head>

<div
  class="min-h-screen bg-[radial-gradient(circle_at_top,_rgba(34,211,238,0.18),_transparent_40%),linear-gradient(180deg,_#f8fafc_0%,_#eef2ff_100%)] px-4 py-8 dark:bg-[radial-gradient(circle_at_top,_rgba(34,211,238,0.12),_transparent_40%),linear-gradient(180deg,_#020617_0%,_#0f172a_100%)]"
>
  <div class="mx-auto max-w-4xl">
    <div class="mb-8">
      <div
        class="mb-4 h-2 overflow-hidden rounded-full bg-white/60 shadow-inner dark:bg-slate-800/90"
      >
        <div
          class="h-full rounded-full bg-cyan-500 transition-all duration-300"
          style={`width:${progress}%`}
        ></div>
      </div>
      <div class="flex items-center justify-between gap-2">
        {#each steps as step, index}
          <div class="flex flex-1 items-center gap-2">
            <div class:active={index <= currentIndex} class="step-dot"></div>
            <span
              class="hidden text-xs text-slate-500 md:inline dark:text-slate-400"
              >{step.label}</span
            >
          </div>
        {/each}
      </div>
    </div>

    {@render children?.()}
  </div>
</div>

<style>
  .step-dot {
    width: 0.8rem;
    height: 0.8rem;
    border-radius: 9999px;
    background: rgb(203 213 225);
    transition:
      transform 150ms ease,
      background-color 150ms ease;
  }

  .step-dot.active {
    background: rgb(6 182 212);
    transform: scale(1.1);
  }

  :global(.dark) .step-dot {
    background: rgb(71 85 105);
  }

  :global(.dark) .step-dot.active {
    background: rgb(34 211 238);
  }
</style>
