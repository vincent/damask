<script lang="ts">
  import { collectionsStore } from '$lib/stores/collections.svelte'
  import { navigationStore } from '$lib/stores/navigation.svelte'
  import { authStore } from '$lib/stores/auth.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { LibraryBig, EllipsisVertical } from '@lucide/svelte'
  import InlineEditForm from '$lib/components/ui/InlineEditForm.svelte'
  import ContextMenu from '$lib/components/ui/ContextMenu.svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    onSelect: (collectionId: string) => void
  }

  let { onSelect }: Props = $props()

  const MAX_VISIBLE = 5
  let expanded = $state(false)
  let menuOpenId = $state<string | null>(null)
  let editingId = $state<string | null>(null)
  let editName = $state('')
  let confirmDeleteId = $state<string | null>(null)

  const visible = $derived(
    expanded ? collectionsStore.collections : collectionsStore.collections.slice(0, MAX_VISIBLE)
  )
  const hiddenCount = $derived(collectionsStore.collections.length - MAX_VISIBLE)

  async function handleRename(id: string, name: string) {
    if (!name.trim()) { editingId = null; return }
    try {
      await collectionsStore.rename(id, name.trim())
    } catch {
      toastStore.show(m.collection_rename_failed(), 'error')
    } finally {
      editingId = null
    }
  }

  async function handleDelete(id: string) {
    confirmDeleteId = null
    try {
      await collectionsStore.remove(id)
      if (navigationStore.activeCollectionId === id) {
        navigationStore.selectCollection(null)
      }
    } catch {
      toastStore.show(m.collection_delete_failed(), 'error')
    }
  }
</script>

{#if collectionsStore.collections.length > 0}
  <div class="flex flex-col gap-0.5">
    {#each visible as col (col.id)}
      <div class="group relative">
        {#if editingId === col.id}
          <div class="px-2 py-1">
            <InlineEditForm
              bind:value={editName}
              onsubmit={(v) => handleRename(col.id, v)}
              oncancel={() => { editingId = null }}
              size="sm"
            />
          </div>
        {:else}
          <button
            class="flex w-full items-center gap-2 rounded-lg px-4 py-1.5 text-md transition-colors
              {navigationStore.activeCollectionId === col.id
                ? 'bg-gray-100 font-medium text-gray-900 dark:bg-gray-700 dark:text-gray-100'
                : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-gray-800'}"
            onclick={() => onSelect(col.id)}
          >
            <LibraryBig class="h-4 w-4 shrink-0 text-gray-400" />
            <span class="min-w-0 flex-1 truncate text-left">{col.name}</span>
            <span class="ml-auto shrink-0 text-sm text-gray-400 p-1">{col.asset_count || ''}</span>
          </button>

          {#if authStore.role !== 'viewer'}
            <button
              class="absolute right-7 top-2.5 rounded p-0.5 text-gray-300 opacity-0 hover:bg-gray-200 hover:text-gray-600 group-hover:opacity-100 dark:text-gray-600 dark:hover:bg-gray-700 dark:hover:text-gray-400"
              onclick={(e) => { e.stopPropagation(); menuOpenId = menuOpenId === col.id ? null : col.id }}
              aria-label="Collection menu"
            >
              <EllipsisVertical class="h-3.5 w-3.5" />
            </button>
          {/if}
        {/if}

        {#if menuOpenId === col.id}
          <div class="absolute right-2 top-full z-20 mt-0.5">
            <ContextMenu
              items={[
                { label: m.rename(), onclick: () => { editingId = col.id; editName = col.name; menuOpenId = null } },
                { label: m.delete_collection(), onclick: () => { confirmDeleteId = col.id; menuOpenId = null }, danger: true },
              ]}
              onclose={() => { menuOpenId = null }}
            />
          </div>
        {/if}
      </div>
    {/each}

    {#if !expanded && hiddenCount > 0}
      <button
        type="button"
        onclick={() => { expanded = true }}
        class="px-2 py-1 text-xs text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 text-right"
      >
        {m.show_more_collections({ count: hiddenCount })}
      </button>
    {/if}
  </div>
{/if}

<!-- Delete confirmation modal -->
{#if confirmDeleteId}
  {@const col = collectionsStore.collections.find(c => c.id === confirmDeleteId)}
  {#if col}
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div class="mx-4 w-full max-w-sm rounded-xl border border-gray-200 bg-white p-5 shadow-2xl dark:border-gray-700 dark:bg-gray-900">
        <p class="text-sm text-gray-700 dark:text-gray-300">
          {m.collection_delete_confirm({ name: col.name, count: col.asset_count })}
        </p>
        <div class="mt-4 flex gap-2 justify-end">
          <button
            type="button"
            onclick={() => { confirmDeleteId = null }}
            class="rounded-lg border border-gray-200 px-3 py-1.5 text-sm text-gray-600 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-400"
          >{m.cancel()}</button>
          <button
            type="button"
            onclick={() => handleDelete(col.id)}
            class="rounded-lg bg-red-500 px-3 py-1.5 text-sm font-medium text-white hover:bg-red-600"
          >{m.delete_collection()}</button>
        </div>
      </div>
    </div>
  {/if}
{/if}
