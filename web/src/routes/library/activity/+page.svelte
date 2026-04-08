<script lang="ts">
  import { activityApi, type ActivityEvent } from '$lib/api'
  import { Bot, ChevronDown, Download, Inbox, User } from '@lucide/svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'

  let events = $state<ActivityEvent[]>([])
  let loading = $state(true)
  let error = $state('')
  let nextCursor = $state<string | null>(null)
  let hasMore = $state(false)
  let loadingMore = $state(false)

  // Filter state
  let typeFilter = $state<'' | 'uploads' | 'changes' | 'shares'>('')

  const typesParam: Record<string, string> = {
    uploads: 'asset_created',
    changes: 'asset_renamed,asset_moved,asset_tagged,asset_untagged,asset_field_set,asset_field_cleared',
    shares: 'asset_shared,asset_share_revoked',
  }

  $effect(() => {
    // Re-fetch when filter changes.
    void typeFilter
    events = []
    nextCursor = null
    hasMore = false
    load()
  })

  async function load() {
    loading = true
    error = ''
    try {
      const res = await activityApi.listWorkspaceActivity({
        limit: 20,
        types: typeFilter ? typesParam[typeFilter] : undefined,
      })
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
      const res = await activityApi.listWorkspaceActivity({
        limit: 20,
        cursor: nextCursor,
        types: typeFilter ? typesParam[typeFilter] : undefined,
      })
      events = [...events, ...res.events]
      nextCursor = res.next_cursor
      hasMore = res.has_more
    } catch { /* silently ignore */ }
    finally { loadingMore = false }
  }

  function relativeTime(isoOrSqlite: string): string {
    const date = new Date(isoOrSqlite.replace(' ', 'T') + (isoOrSqlite.includes('T') ? '' : 'Z'))
    const diffSec = Math.floor((Date.now() - date.getTime()) / 1000)
    if (diffSec < 60) return 'just now'
    if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m ago`
    if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h ago`
    if (diffSec < 86400 * 7) return `${Math.floor(diffSec / 86400)}d ago`
    return date.toLocaleDateString()
  }

  const csvUrl = $derived(activityApi.exportCSV())
</script>

<svelte:head>
  <title>Activity — Damask</title>
</svelte:head>

<div class="flex h-full flex-col overflow-hidden">
  <!-- Header -->
  <div class="flex flex-shrink-0 items-center justify-between border-b border-zinc-100 px-6 py-4 dark:border-zinc-800">
    <div>
      <h1 class="text-xl font-semibold text-zinc-900 dark:text-zinc-100">Workspace Activity</h1>
      <p class="text-md text-zinc-500 dark:text-zinc-400">Audit trail of all asset and project changes</p>
    </div>
    <a
      href={csvUrl}
      class="flex items-center gap-2 rounded-lg border border-zinc-200 px-3 py-1.5 text-md text-zinc-600 transition-colors hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-400 dark:hover:bg-zinc-800"
    >
      <Download class="h-4 w-4" />
      Export CSV
    </a>
  </div>

  <!-- Filter bar -->
  <div class="flex flex-shrink-0 gap-2 border-b border-zinc-100 px-6 py-2 dark:border-zinc-800">
    {#each ([['', 'All'], ['uploads', 'Uploads'], ['changes', 'Changes'], ['shares', 'Shares']] as [string, string][]) as [val, label]}
      <button
        type="button"
        class="rounded-full px-3 py-1 text-sm font-medium transition-colors
          {typeFilter === val
            ? 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/50 dark:text-indigo-300'
            : 'text-zinc-500 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-800'}"
        onclick={() => { typeFilter = val as typeof typeFilter }}
      >
        {label}
      </button>
    {/each}
  </div>

  <!-- Content -->
  <div class="flex-1 overflow-y-auto">
    {#if loading}
      <div class="flex justify-center py-16">
        <Spinner size="md" />
      </div>

    {:else if error}
      <p class="px-6 py-8 text-md text-red-500">{error}</p>

    {:else if events.length === 0}
      <div class="flex flex-col items-center gap-3 py-16 text-center text-zinc-400 dark:text-zinc-500">
        <Inbox class="h-10 w-10" />
        <p class="text-md">No activity yet.</p>
      </div>

    {:else}
      <ul class="divide-y divide-zinc-100 dark:divide-zinc-800">
        {#each events as event (event.id)}
          <li class="flex gap-4 px-6 py-4">
            <!-- Actor icon -->
            <div class="mt-0.5 flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full
              {event.actor.type === 'system'
                ? 'bg-zinc-100 text-zinc-400 dark:bg-zinc-800 dark:text-zinc-500'
                : 'bg-indigo-100 text-indigo-600 dark:bg-indigo-900/50 dark:text-indigo-400'}">
              {#if event.actor.type === 'system'}
                <Bot class="h-4 w-4" />
              {:else}
                <User class="h-4 w-4" />
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
              <div class="mt-0.5 flex items-center gap-2 text-sm text-zinc-400 dark:text-zinc-500">
                <span class="rounded bg-zinc-100 px-1.5 py-0.5 font-mono dark:bg-zinc-800">
                  {event.entity_type}
                </span>
                <span>{relativeTime(event.created_at)}</span>
              </div>
            </div>
          </li>
        {/each}
      </ul>

      {#if hasMore}
        <div class="border-t border-zinc-100 px-6 py-4 dark:border-zinc-800">
          <button
            type="button"
            class="flex w-full items-center justify-center gap-2 text-md text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
            onclick={loadMore}
            disabled={loadingMore}
          >
            {#if loadingMore}
              <Spinner size="sm" />
            {:else}
              <ChevronDown class="h-4 w-4" />
              Load more
            {/if}
          </button>
        </div>
      {/if}
    {/if}
  </div>
</div>
