<script lang="ts">
  import { Play, Pencil, Trash2, ToggleLeft, ToggleRight, AlertCircle, CheckCircle, Clock } from '@lucide/svelte'
  import type { IngressSource } from '$lib/api/models'
  import { ingressStore } from '$lib/stores/ingress.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import SourceTypeIcon from './SourceTypeIcon.svelte'

  interface Props {
    source: IngressSource
    onedit: (source: IngressSource) => void
    ondelete: (source: IngressSource) => void
  }

  let { source, onedit, ondelete }: Props = $props()

  const SOURCE_LABELS: Record<string, string> = {
    email_api: 'Email address',
    imap: 'IMAP mailbox',
    sftp: 'SFTP server',
    dav: 'WebDAV / Nextcloud',
    s3: 'S3-compatible bucket',
  }

  const destProject = $derived(
    source.dest_project_id
      ? projectsStore.projects.find((p) => p.id === source.dest_project_id)
      : null,
  )

  function formatRelative(iso: string | null): string {
    if (!iso) return 'Never'
    const diff = Date.now() - new Date(iso).getTime()
    const mins = Math.floor(diff / 60_000)
    if (mins < 1) return 'Just now'
    if (mins < 60) return `${mins}m ago`
    const hrs = Math.floor(mins / 60)
    if (hrs < 24) return `${hrs}h ago`
    return `${Math.floor(hrs / 24)}d ago`
  }

  let polling = $state(false)
  async function handlePoll() {
    polling = true
    await ingressStore.pollSource(source.id)
    polling = false
  }
</script>

<div class="overflow-hidden rounded-xl border border-zinc-200 bg-white dark:divide-zinc-800 dark:border-zinc-700 dark:bg-zinc-900">
  <div class="flex items-start gap-4 px-5 py-4">
    <!-- Icon -->
    <div class="mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-indigo-50 text-indigo-500 dark:bg-indigo-900/30 dark:text-indigo-400">
      <SourceTypeIcon type={source.type} class="h-5 w-5" />
    </div>

    <!-- Meta -->
    <div class="min-w-0 flex-1">
      <div class="flex flex-wrap items-center gap-2">
        <span class="text-md font-semibold text-gray-900 dark:text-gray-50">{source.label}</span>
        <span class="text-sm text-gray-400 dark:text-gray-500">{SOURCE_LABELS[source.type] ?? source.type}</span>

        <!-- Status pill -->
        {#if !source.enabled}
          <span class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-500 dark:bg-gray-800 dark:text-gray-400">
            Disabled
          </span>
        {:else if source.last_error}
          <span class="inline-flex items-center gap-1 rounded-full bg-red-100 px-2 py-0.5 text-[11px] font-medium text-red-600 dark:bg-red-900/30 dark:text-red-400">
            <AlertCircle class="h-3 w-3" /> Error
          </span>
        {:else}
          <span class="inline-flex items-center gap-1 rounded-full bg-green-100 px-2 py-0.5 text-[11px] font-medium text-green-700 dark:bg-green-900/30 dark:text-green-400">
            <CheckCircle class="h-3 w-3" /> Healthy
          </span>
        {/if}
      </div>

      <!-- Destination + last polled -->
      <div class="mt-1 flex flex-wrap items-center gap-3 text-sm text-gray-400 dark:text-gray-500">
        {#if destProject}
          <span>→ {destProject.name}</span>
        {/if}
        <span class="inline-flex items-center gap-1">
          <Clock class="h-3 w-3" />
          {formatRelative(source.last_polled_at)}
        </span>
        <span>Every {source.poll_interval_min}m</span>
      </div>

      {#if source.last_error}
        <p class="mt-1.5 truncate text-sm text-red-500 dark:text-red-400">{source.last_error}</p>
      {/if}
    </div>

    <!-- Actions -->
    <div class="flex shrink-0 items-center gap-1">
      <!-- Poll now -->
      <button
        type="button"
        disabled={polling || !source.enabled}
        class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 disabled:cursor-not-allowed disabled:opacity-40 dark:hover:bg-gray-800 dark:hover:text-gray-200"
        onclick={handlePoll}
        title="Poll now"
      >
        <Play class="h-4 w-4" />
      </button>

      <!-- Toggle enable/disable -->
      <button
        type="button"
        class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-gray-800 dark:hover:text-gray-200"
        onclick={() => ingressStore.toggleSource(source.id, !source.enabled)}
        title={source.enabled ? 'Disable' : 'Enable'}
      >
        {#if source.enabled}
          <ToggleRight class="h-4 w-4 text-indigo-500" />
        {:else}
          <ToggleLeft class="h-4 w-4" />
        {/if}
      </button>

      <!-- Edit -->
      <button
        type="button"
        class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-gray-800 dark:hover:text-gray-200"
        onclick={() => onedit(source)}
        title="Edit"
      >
        <Pencil class="h-4 w-4" />
      </button>

      <!-- Delete -->
      <button
        type="button"
        class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
        onclick={() => ondelete(source)}
        title="Delete"
      >
        <Trash2 class="h-4 w-4" />
      </button>
    </div>
  </div>
</div>
