<script lang="ts">
  import { onMount } from 'svelte'
  import { Plus, Rss } from '@lucide/svelte'
  import { ingressStore } from '$lib/stores/ingress.svelte'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import type { IngressSource } from '$lib/api/models'
  import IngressSourceCard from '$lib/components/ingress/IngressSourceCard.svelte'
  import AddSourceModal from '$lib/components/ingress/AddSourceModal.svelte'
  import SourceDetailPanel from '$lib/components/ingress/SourceDetailPanel.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import EmptyState from '$lib/components/ui/EmptyState.svelte'
  import GridSkeleton from '$lib/components/ui/GridSkeleton.svelte'

  let showAddModal = $state(false)
  let editingSource = $state<IngressSource | null>(null)
  let detailSource = $state<IngressSource | null>(null)

  onMount(() => {
    ingressStore.loadSources()
    projectsStore.load()
  })

  function handleEdit(source: IngressSource) {
    editingSource = source
    detailSource = source
  }

  async function handleDelete(source: IngressSource) {
    if (!confirm(`Delete "${source.label}"? This will also remove all its log entries.`)) return
    const ok = await ingressStore.deleteSource(source.id)
    if (ok && detailSource?.id === source.id) detailSource = null
  }

  function handleAdded(source: IngressSource) {
    showAddModal = false
    detailSource = source
  }
</script>

<svelte:head>
  <title>Ingress Sources — Damask</title>
</svelte:head>

<div class="flex flex-1 overflow-hidden">
  <!-- Source list -->
  <main class="flex-1 overflow-y-auto px-6 py-6">
    {#if ingressStore.loadingSources}
      <GridSkeleton lines={4} />

    {:else if ingressStore.sources.length === 0}
      <EmptyState
        title="No sources yet"
        description="Add your first ingress source to automatically import files from email, SFTP, S3, and more."
      >
        {#snippet icon()}
          <Rss class="h-10 w-10" />
        {/snippet}
        {#snippet action()}
          <Button variant="primary" onclick={() => { showAddModal = true }}>
            {#snippet icon()}<Plus class="h-4 w-4" />{/snippet}
            Add your first source
          </Button>
        {/snippet}
      </EmptyState>

    {:else}
      <div class="space-y-3">
        {#each ingressStore.sources as source (source.id)}
          <button
            type="button"
            class="w-full text-left focus:outline-none"
            onclick={() => { detailSource = source }}
          >
            <IngressSourceCard
              {source}
              onedit={handleEdit}
              ondelete={handleDelete}
            />
          </button>
        {/each}
      </div>
      <div class="mt-4 flex justify-center">
        <Button variant="primary" onclick={() => { showAddModal = true }}>
          {#snippet icon()}<Plus class="h-4 w-4" />{/snippet}
          Add source
        </Button>
      </div>
    {/if}
  </main>

  <!-- Detail panel (right side) -->
  {#if detailSource}
    <SourceDetailPanel
      source={detailSource}
      onclose={() => { detailSource = null }}
      onupdated={(s) => { detailSource = s }}
    />
  {/if}
</div>

{#if showAddModal}
  <AddSourceModal
    bind:open={showAddModal}
    onadded={handleAdded}
    onclose={() => { showAddModal = false }}
  />
{/if}
