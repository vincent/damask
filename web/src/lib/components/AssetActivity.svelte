<script lang="ts">
  import { activityApi, type Asset, type AuditEvent } from '$lib/api'
  import { Activity, Bot, ChevronDown, Inbox, User } from '@lucide/svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'

  interface Props {
    asset: Asset
  }

  let { asset }: Props = $props()

  let events = $state<AuditEvent[]>([])
  let loading = $state(false)
  let error = $state('')
  let nextCursor = $state<string | null>(null)
  let hasMore = $state(false)
  let loadingMore = $state(false)

  $effect(() => {
    if (asset?.id) {
      events = []
      nextCursor = null
      hasMore = false
      load()
    }
  })

  async function load() {
    loading = true
    error = ''
    try {
      const res = await activityApi.listAssetEvents(asset.id, { limit: 50 })
      events = res.events
      nextCursor = res.next_cursor
      hasMore = res.has_more
    } catch (e: unknown) {
      error = e instanceof Error ? e.message : 'Failed to load activity'
    } finally {
      loading = false
    }
  }

  async function loadMore() {
    if (!nextCursor || loadingMore) return
    loadingMore = true
    try {
      const res = await activityApi.listAssetEvents(asset.id, { limit: 50, cursor: nextCursor })
      events = [...events, ...res.events]
      nextCursor = res.next_cursor
      hasMore = res.has_more
    } catch { /* silently ignore */ }
    finally {
      loadingMore = false
    }
  }

  function relativeTime(isoOrSqlite: string): string {
    const date = new Date(isoOrSqlite.replace(' ', 'T') + (isoOrSqlite.includes('T') ? '' : 'Z'))
    const diffMs = Date.now() - date.getTime()
    const diffSec = Math.floor(diffMs / 1000)
    if (diffSec < 60) return 'just now'
    if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m ago`
    if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h ago`
    if (diffSec < 86400 * 7) return `${Math.floor(diffSec / 86400)}d ago`
    return date.toLocaleDateString()
  }
</script>

<div class="flex flex-col">
  {#if loading}
    <div class="flex justify-center py-12">
      <Spinner size="md" />
    </div>

  {:else if error}
    <p class="px-5 py-6 text-md text-red-500">{error}</p>

  {:else if events.length === 0}
    <div class="flex flex-col items-center gap-3 py-12 text-center text-zinc-400 dark:text-zinc-500">
      <Inbox class="h-10 w-10" />
      <p class="text-md">No activity yet.</p>
    </div>

  {:else}
    <ul class="divide-y divide-zinc-100 dark:divide-zinc-800">
      {#each events as event (event.id)}
        <li class="px-5 py-3 flex gap-3 items-start">
          <!-- Actor icon -->
          <div class="mt-0.5 flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full
            {event.actor.type === 'system'
              ? 'bg-zinc-100 text-zinc-400 dark:bg-zinc-800 dark:text-zinc-500'
              : 'bg-indigo-100 text-indigo-600 dark:bg-indigo-900/50 dark:text-indigo-400'}">
            {#if event.actor.type === 'system'}
              <Bot class="h-3.5 w-3.5" />
            {:else}
              <User class="h-3.5 w-3.5" />
            {/if}
          </div>

          <!-- Content -->
          <div class="min-w-0 flex-1">
            <p class="text-md text-zinc-800 dark:text-zinc-200">
              {#if event.actor.type === 'user' && event.actor.name}
                <span class="font-medium">{event.actor.name}</span>
                {' '}
              {:else if event.actor.type === 'system'}
                <span class="font-medium italic text-zinc-500 dark:text-zinc-400">System</span>
                {' '}
              {/if}
              <span class="text-zinc-600 dark:text-zinc-400">{event.human_readable}</span>
            </p>
            <p class="mt-0.5 text-sm text-zinc-400 dark:text-zinc-500">
              {relativeTime(event.created_at)}
            </p>
          </div>
        </li>
      {/each}
    </ul>

    {#if hasMore}
      <div class="border-t border-zinc-100 px-5 py-3 dark:border-zinc-800">
        <button
          type="button"
          class="flex w-full items-center justify-center gap-1.5 text-sm text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
          onclick={loadMore}
          disabled={loadingMore}
        >
          {#if loadingMore}
            <Spinner size="sm" />
          {:else}
            <ChevronDown class="h-3.5 w-3.5" />
            Load more
          {/if}
        </button>
      </div>
    {/if}
  {/if}
</div>
