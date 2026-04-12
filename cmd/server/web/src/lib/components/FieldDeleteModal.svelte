<script lang="ts">
  import { fieldDefinitionApi } from '$lib/api/client'
  import type { FieldDefinition } from '$lib/api/models'
  import Modal from '$lib/components/ui/Modal.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import Feedback from './ui/Feedback.svelte'

  interface Props {
    open?: boolean
    field: FieldDefinition | null
    onclose: () => void
    ondeleted: (id: string) => void
  }

  let { open = $bindable(false), field, onclose, ondeleted }: Props = $props()

  let stats = $state<{ asset_count: number; project_count: number } | null>(null)
  let statsLoading = $state(false)
  let deleting = $state(false)
  let error = $state('')

  $effect(() => {
    if (open && field) {
      stats = null
      error = ''
      statsLoading = true
      fieldDefinitionApi.stats(field.id).then((s) => {
        stats = s
      }).catch(() => {
        stats = { asset_count: 0, project_count: 0 }
      }).finally(() => {
        statsLoading = false
      })
    }
  })

  async function handleDelete() {
    if (!field) return
    deleting = true
    error = ''
    try {
      await fieldDefinitionApi.delete(field.id)
      ondeleted(field.id)
      open = false
    } catch (e: unknown) {
      error = e instanceof Error ? e.message : 'Could not delete field'
    } finally {
      deleting = false
    }
  }
</script>

<Modal bind:open {onclose}>
  <div class="p-6">
    <h2 class="mb-2 text-base font-semibold text-gray-900 dark:text-gray-100">
      Delete "{field?.name}"?
    </h2>

    {#if statsLoading}
      <div class="flex justify-center py-6">
        <Spinner size="sm" />
      </div>
    {:else if stats}
      <p class="text-md text-gray-600 dark:text-gray-400">
        {#if stats.asset_count > 0 || stats.project_count > 0}
          This field has values set on
          {#if stats.asset_count > 0}
            <strong class="text-gray-900 dark:text-gray-100">{stats.asset_count} asset{stats.asset_count !== 1 ? 's' : ''}</strong>
          {/if}
          {#if stats.asset_count > 0 && stats.project_count > 0} and {/if}
          {#if stats.project_count > 0}
            <strong class="text-gray-900 dark:text-gray-100">{stats.project_count} project{stats.project_count !== 1 ? 's' : ''}</strong>
          {/if}.
          The values will be kept for 30 days and then permanently deleted.
        {:else}
          This field has no values set. It will be soft-deleted and permanently removed after 30 days.
        {/if}
      </p>
    {/if}

    <Feedback {error} />

    <div class="mt-5 flex justify-end gap-2">
      <Button variant="secondary" onclick={onclose}>Cancel</Button>
      <Button variant="danger" loading={deleting} onclick={handleDelete}>Delete field</Button>
    </div>
  </div>
</Modal>
