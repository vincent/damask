<script lang="ts">
  import { onMount } from 'svelte'
  import { Plus, Upload } from '@lucide/svelte'
  import { exportsApi, type ExportConfig } from '$lib/api/exports'
  import { projectsStore } from '$lib/stores/projects.svelte'
  import ExportConfigCard from '$lib/components/exports/ExportConfigCard.svelte'
  import ExportConfigModal from '$lib/components/exports/ExportConfigModal.svelte'
  import PageHeader from '$lib/components/ui/PageHeader.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import EmptyState from '$lib/components/ui/EmptyState.svelte'
  import GridSkeleton from '$lib/components/ui/GridSkeleton.svelte'

  let configs = $state<ExportConfig[]>([])
  let loading = $state(true)
  let showModal = $state(false)
  let editingConfig = $state<ExportConfig | null>(null)
  let confirmDeleteConfig = $state<ExportConfig | null>(null)

  onMount(async () => {
    projectsStore.load()
    try {
      configs = await exportsApi.list()
    } finally {
      loading = false
    }
  })

  function openCreate() {
    editingConfig = null
    showModal = true
  }

  function openEdit(config: ExportConfig) {
    editingConfig = config
    showModal = true
  }

  function handleSaved(config: ExportConfig) {
    const idx = configs.findIndex((c) => c.id === config.id)
    if (idx >= 0) {
      configs[idx] = config
    } else {
      configs = [...configs, config]
    }
    showModal = false
  }

  function handleDelete(config: ExportConfig) {
    confirmDeleteConfig = config
  }

  async function confirmDelete() {
    if (!confirmDeleteConfig) return
    await exportsApi.delete(confirmDeleteConfig.id)
    configs = configs.filter((c) => c.id !== confirmDeleteConfig!.id)
    confirmDeleteConfig = null
  }

  async function handleToggle(config: ExportConfig) {
    const updated = await exportsApi.update(config.id, {
      enabled: !config.enabled,
    })
    const idx = configs.findIndex((c) => c.id === config.id)
    if (idx >= 0) configs[idx] = updated
  }
</script>

<svelte:head>
  <title>Exports — Damask</title>
</svelte:head>

<div class="flex flex-1 flex-col overflow-hidden">
  <PageHeader
    title="Exports"
    description="Automatically ship project assets to remote destinations on a schedule or on demand."
  >
    <Button variant="primary" onclick={openCreate}>
      {#snippet icon()}<Plus class="h-4 w-4" />{/snippet}
      New export
    </Button>
  </PageHeader>

  <div class="flex-1 overflow-y-auto px-6 py-6">
    {#if loading}
      <GridSkeleton lines={3} />
    {:else if configs.length === 0}
      <EmptyState
        title="No exports yet"
        description="Create an export to ship project assets to an SFTP server or Google Drive folder."
      >
        {#snippet action()}
          <Button variant="primary" onclick={openCreate}>
            {#snippet icon()}<Upload class="h-4 w-4" />{/snippet}
            New export
          </Button>
        {/snippet}
      </EmptyState>
    {:else}
      <div class="max-w-2xl space-y-3">
        {#each configs as config (config.id)}
          <ExportConfigCard
            {config}
            onEdit={openEdit}
            onDelete={handleDelete}
            onToggle={handleToggle}
          />
        {/each}
      </div>
    {/if}
  </div>
</div>

<ExportConfigModal
  bind:open={showModal}
  config={editingConfig}
  onSave={handleSaved}
  onClose={() => (showModal = false)}
/>

{#if confirmDeleteConfig}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4 backdrop-blur-sm"
  >
    <div
      class="mx-4 w-full max-w-sm rounded-xl border border-gray-100 bg-white p-6 shadow-xl dark:border-gray-800 dark:bg-gray-900"
    >
      <p class="mb-2 font-semibold text-gray-900 dark:text-gray-100">
        Delete export config?
      </p>
      <p class="mb-5 text-sm text-gray-500 dark:text-gray-400">
        Files already written to the destination are not affected.
      </p>
      <div class="flex justify-end gap-2">
        <Button
          variant="secondary"
          size="sm"
          onclick={() => (confirmDeleteConfig = null)}
        >
          Cancel
        </Button>
        <Button variant="danger" size="sm" onclick={confirmDelete}
          >Delete</Button
        >
      </div>
    </div>
  </div>
{/if}
