<script lang="ts">
  const API_BASE = import.meta.env.VITE_API_URL ?? ''
  import { Bot } from '@lucide/svelte'
  import { avatarColors, initialsFor } from '$lib/utils/avatar'

  let {
    actor = null,
    class: extraClass,
  }: {
    actor: { id?: string | null; name?: string | null } | null
    class?: string
  } = $props()

  let imgError = $state(false)

  const initials = $derived(initialsFor(actor?.name, actor?.id ?? '?'))
  const [fg, bg] = $derived.by(() =>
    avatarColors(actor?.name || actor?.id || '')
  )
</script>

{#if actor?.id}
  <span class="actor-avatar {extraClass}" style="--fg: {fg}; --bg: {bg}">
    {#if !imgError}
      <img
        src={`${API_BASE}/api/v1/users/${actor.id}/avatar`}
        alt={actor.name || actor.id.split('-')[0]}
        class="actor-avatar-img"
        onerror={() => {
          imgError = true
        }}
      />
    {:else}
      {initials}
    {/if}
  </span>
{:else}
  <Bot
    class="bg-zinc-100 text-zinc-400 dark:bg-zinc-800 dark:text-zinc-500 {extraClass}"
  />
{/if}

<style>
  .actor-avatar {
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    overflow: hidden;
    background: var(--bg);
    color: var(--fg);
    font-size: 0.65rem;
    font-weight: 600;
    letter-spacing: 0.02em;
    user-select: none;
  }

  .actor-avatar-img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    border-radius: 50%;
  }
</style>
