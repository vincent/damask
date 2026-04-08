<script lang="ts">
  import { onMount } from 'svelte'
  import { GripVertical, Pencil, Trash2, Plus, LayoutList } from '@lucide/svelte'
  import { customFieldsStore } from '$lib/stores/customFields.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import type { FieldDefinition, FieldScope } from '$lib/api/models'
  import FieldCreateEditModal from '$lib/components/FieldCreateEditModal.svelte'
  import FieldDeleteModal from '$lib/components/FieldDeleteModal.svelte'
  import Badge from '$lib/components/ui/Badge.svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import EmptyState from '$lib/components/ui/EmptyState.svelte'

  let activeScope = $state<FieldScope>('asset')

  let showCreateModal = $state(false)
  let editingField = $state<FieldDefinition | null>(null)
  let deletingField = $state<FieldDefinition | null>(null)
  let showDeleteModal = $state(false)

  // Drag-to-reorder state
  let dragIndex = $state<number | null>(null)
  let dragOverIndex = $state<number | null>(null)

  onMount(() => {
    customFieldsStore.loadBoth()
  })

  const fields = $derived(customFieldsStore.fields(activeScope))

  function handleEdit(field: FieldDefinition) {
    editingField = field
    showCreateModal = true
  }

  function handleDelete(field: FieldDefinition) {
    deletingField = field
    showDeleteModal = true
  }

  function handleSaved(def: FieldDefinition) {
    customFieldsStore.upsertLocal(activeScope, def)
    toastStore.show(editingField ? 'Field updated' : 'Field created')
    editingField = null
    showCreateModal = false
  }

  function handleDeleted(id: string) {
    customFieldsStore.removeLocal(activeScope, id)
    toastStore.show('Field deleted')
    deletingField = null
    showDeleteModal = false
  }

  // Drag reorder handlers
  function dragStart(i: number) {
    dragIndex = i
  }

  function dragOver(e: DragEvent, i: number) {
    e.preventDefault()
    dragOverIndex = i
  }

  async function drop() {
    if (dragIndex === null || dragOverIndex === null || dragIndex === dragOverIndex) {
      dragIndex = null
      dragOverIndex = null
      return
    }
    const reordered = [...fields]
    const [moved] = reordered.splice(dragIndex, 1)
    reordered.splice(dragOverIndex, 0, moved)
    dragIndex = null
    dragOverIndex = null
    try {
      await customFieldsStore.reorder(activeScope, reordered)
    } catch {
      toastStore.show('Could not save order', 'error')
      await customFieldsStore.load(activeScope)
    }
  }

  function fieldTypeBadge(type: string): string {
    const map: Record<string, string> = {
      text: 'Text', number: 'Number', date: 'Date',
      boolean: 'Yes/No', select: 'Select', url: 'URL',
    }
    return map[type] ?? type
  }
</script>

<svelte:head>
  <title>Custom Fields — Damask</title>
</svelte:head>

<div class="flex h-full flex-col">
  <!-- Page header -->
  <div class="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-gray-800">
    <div class="flex items-center gap-3">
      <LayoutList class="h-5 w-5 text-gray-400" />
      <h1 class="text-base font-semibold text-gray-900 dark:text-gray-100">Custom Fields</h1>
    </div>
    <button
      class="flex items-center gap-1.5 rounded-lg bg-indigo-600 px-3 py-1.5 text-md font-medium text-white hover:bg-indigo-700 dark:bg-indigo-500 dark:hover:bg-indigo-600"
      onclick={() => { editingField = null; showCreateModal = true }}
    >
      <Plus class="h-4 w-4" />
      Add field
    </button>
  </div>

  <!-- Scope tabs -->
  <div class="flex border-b border-gray-100 px-6 dark:border-gray-800">
    {#each (['asset', 'project'] as FieldScope[]) as scope}
      <button
        type="button"
        class="relative py-3 pr-6 text-md font-medium transition-colors
          {activeScope === scope
            ? 'text-indigo-600 dark:text-indigo-400'
            : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'}"
        onclick={() => { activeScope = scope }}
      >
        {scope === 'asset' ? 'Asset fields' : 'Project fields'}
        {#if activeScope === scope}
          <span class="absolute bottom-0 left-0 right-4 h-0.5 rounded-t bg-indigo-600 dark:bg-indigo-400"></span>
        {/if}
      </button>
    {/each}
  </div>

  <!-- Field list -->
  <div class="flex-1 overflow-y-auto px-6 py-4">
    {#if customFieldsStore.loading}
      <div class="flex justify-center py-16">
        <Spinner size="md" />
      </div>
    {:else if fields.length === 0}
      <EmptyState
        title="No {activeScope} fields yet"
        description="Add custom metadata fields to collect structured data on your {activeScope === 'asset' ? 'assets' : 'projects'}."
      />
    {:else}
      <ul class="space-y-2">
        {#each fields as field, i (field.id)}
          <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
          <li
            class="flex items-center gap-3 rounded-xl border bg-white px-4 py-3 transition-colors dark:bg-gray-900
              {dragOverIndex === i ? 'border-indigo-400 dark:border-indigo-500' : 'border-gray-200 dark:border-gray-700'}"
            draggable="true"
            ondragstart={() => dragStart(i)}
            ondragover={(e) => dragOver(e, i)}
            ondrop={drop}
            ondragend={() => { dragIndex = null; dragOverIndex = null }}
          >
            <!-- Drag handle -->
            <GripVertical class="h-4 w-4 shrink-0 cursor-grab text-gray-300 dark:text-gray-600" />

            <!-- Field info -->
            <div class="flex flex-1 flex-wrap items-center gap-2 min-w-0">
              <span class="text-md font-medium text-gray-900 dark:text-gray-100">{field.name}</span>
              <Badge>{fieldTypeBadge(field.field_type)}</Badge>
              <span class="font-mono text-[11px] text-gray-400 dark:text-gray-500">{field.key}</span>
              {#if field.required}
                <span class="text-[11px] font-medium text-orange-500 dark:text-orange-400">Required</span>
              {/if}
              {#if field.inherit_from_project && activeScope === 'asset'}
                <span class="text-[11px] text-gray-400 dark:text-gray-500">auto-fills assets</span>
              {/if}
            </div>

            <!-- Options preview for select -->
            {#if field.field_type === 'select' && field.options}
              {@const opts = JSON.parse(field.options) as string[]}
              <div class="flex flex-wrap gap-1">
                {#each opts.slice(0, 4) as opt}
                  <span class="rounded bg-gray-100 px-1.5 py-0.5 text-[11px] text-gray-600 dark:bg-gray-800 dark:text-gray-400">{opt}</span>
                {/each}
                {#if opts.length > 4}
                  <span class="text-[11px] text-gray-400">+{opts.length - 4}</span>
                {/if}
              </div>
            {/if}

            <!-- Actions -->
            <div class="flex shrink-0 items-center gap-1">
              <button
                class="rounded p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-800 dark:hover:text-gray-200"
                onclick={() => handleEdit(field)}
                aria-label="Edit {field.name}"
              >
                <Pencil class="h-3.5 w-3.5" />
              </button>
              <button
                class="rounded p-1.5 text-gray-400 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-950/30 dark:hover:text-red-400"
                onclick={() => handleDelete(field)}
                aria-label="Delete {field.name}"
              >
                <Trash2 class="h-3.5 w-3.5" />
              </button>
            </div>
          </li>
        {/each}
      </ul>
      <p class="mt-3 text-sm text-gray-400 dark:text-gray-600">Drag rows to reorder.</p>
    {/if}

    <div class="flex justify-center items-center">
      <button
        class="flex items-center gap-1.5 rounded-lg bg-indigo-600 px-3 py-1.5 text-md font-medium text-white hover:bg-indigo-700 dark:bg-indigo-500 dark:hover:bg-indigo-600"
        onclick={() => { editingField = null; showCreateModal = true }}
      >
        <Plus class="h-4 w-4" />
        Add field
      </button>
    </div>
  </div>
</div>

<FieldCreateEditModal
  bind:open={showCreateModal}
  scope={activeScope}
  editing={editingField}
  onclose={() => { showCreateModal = false; editingField = null }}
  onsaved={handleSaved}
/>

<FieldDeleteModal
  bind:open={showDeleteModal}
  field={deletingField}
  onclose={() => { showDeleteModal = false; deletingField = null }}
  ondeleted={handleDeleted}
/>
