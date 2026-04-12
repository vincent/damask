<script lang="ts">
  import { Inbox } from '@lucide/svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import Feedback from './ui/Feedback.svelte'
  import ButtonShowMore from './ui/ButtonShowMore.svelte'
  import Actor from './ui/Actor.svelte'
  import type { AuditEvent } from '$lib/api'

  let { loading, error, events, hasMore, loadMore, loadingMore, class: extraClass }: { loading: boolean; error: string; events: AuditEvent[]; hasMore: boolean; loadMore: () => void; loadingMore: boolean; class?: string } = $props()

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

<div class={`flex flex-col ${extraClass}`}>
  {#if loading}
    <div class="flex justify-center py-12">
      <Spinner size="md" />
    </div>

  {:else if error}
    <Feedback {error} />

  {:else if events.length === 0}
    <div class="flex flex-col items-center gap-3 py-12 text-center text-zinc-400 dark:text-zinc-500">
      <Inbox class="h-10 w-10" />
      <p class="text-md">No activity yet.</p>
    </div>

  {:else}
    <ul class="divide-y divide-zinc-100 dark:divide-zinc-800">
      {#each events as event (event.id)}
        <li class="px-5 py-3 flex gap-3 items-start">
          <Actor type={event.actor.type} />
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
      <ButtonShowMore onclick={loadMore} loading={loadingMore} />
    {/if}
  {/if}
</div>
