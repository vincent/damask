<script lang="ts">
  import { fieldDefinitionApi, projectFieldApi } from '$lib/api/client'
  import type { FieldDefinition, ProjectFieldValue } from '$lib/api/models'
  import { Check, ChevronDown, ChevronRight, ArrowDownToLine } from '@lucide/svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'

  interface Props {
    projectId: string
  }

  let { projectId }: Props = $props()

  let definitions = $state<FieldDefinition[]>([])
  let values = $state<ProjectFieldValue[]>([])
  let loading = $state(true)
  let showDeprecated = $state(false)

  let editingFieldId = $state<string | null>(null)
  let editValue = $state<string>('')
  let savingFieldId = $state<string | null>(null)
  let saveSuccess = $state<string | null>(null)
  let saveError = $state<string | null>(null)

  $effect(() => {
    if (projectId) load()
  })

  async function load() {
    loading = true
    try {
      const [defs, vals] = await Promise.all([
        fieldDefinitionApi.list('project'),
        projectFieldApi.get(projectId),
      ])
      definitions = defs
      values = vals.fields
    } finally {
      loading = false
    }
  }

  function valueFor(fieldId: string): ProjectFieldValue | undefined {
    return values.find((v) => v.field_id === fieldId)
  }

  function displayValue(fv: ProjectFieldValue): string {
    if (fv.value === null || fv.value === undefined) return ''
    if (fv.field_type === 'boolean') return fv.value ? 'Yes' : 'No'
    return String(fv.value)
  }

  function startEdit(def: FieldDefinition) {
    const fv = valueFor(def.id)
    editingFieldId = def.id
    saveError = null
    saveSuccess = null
    if (fv && fv.value !== null && fv.value !== undefined) {
      editValue = def.field_type === 'boolean' ? (fv.value ? 'true' : 'false') : String(fv.value)
    } else {
      editValue = def.field_type === 'boolean' ? 'false' : ''
    }
  }

  async function saveField(def: FieldDefinition) {
    savingFieldId = def.id
    saveError = null
    saveSuccess = null
    try {
      let parsedValue: string | number | boolean | null = null

      if (editValue === '' || editValue === null) {
        parsedValue = null
      } else if (def.field_type === 'number') {
        const n = parseFloat(editValue)
        if (isNaN(n)) { saveError = 'Must be a number'; savingFieldId = null; return }
        parsedValue = n
      } else if (def.field_type === 'boolean') {
        parsedValue = editValue === 'true'
      } else {
        parsedValue = editValue
      }

      const result = await projectFieldApi.patch(projectId, [{ field_id: def.id, value: parsedValue }])
      values = result.fields
      editingFieldId = null
      saveSuccess = def.id
      setTimeout(() => { if (saveSuccess === def.id) saveSuccess = null }, 2000)
    } catch (e: unknown) {
      saveError = e instanceof Error ? e.message : 'Could not save'
    } finally {
      savingFieldId = null
    }
  }

  function cancelEdit() {
    editingFieldId = null
    saveError = null
  }

  function handleKeydown(e: KeyboardEvent, def: FieldDefinition) {
    if (e.key === 'Enter' && def.field_type !== 'text') {
      e.preventDefault()
      saveField(def)
    }
    if (e.key === 'Escape') cancelEdit()
  }

  const activeDefinitions = $derived(definitions.filter((d) => !d.deleted_at))
  const orphanedValues = $derived(values.filter((v) => v.definition_deleted))
</script>

<div>
  <p class="mb-3 text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">
    Custom Fields
  </p>

  {#if loading}
    <div class="flex justify-center py-4">
      <Spinner size="sm" />
    </div>
  {:else if activeDefinitions.length === 0}
    <p class="text-xs text-gray-400 dark:text-gray-500">No project fields defined yet.</p>
  {:else}
    <div class="space-y-2">
      {#each activeDefinitions as def (def.id)}
        {@const fv = valueFor(def.id)}
        {@const isEditing = editingFieldId === def.id}
        {@const isSaving = savingFieldId === def.id}
        {@const didSave = saveSuccess === def.id}

        <div class="rounded-xl border border-gray-100 bg-gray-50 px-3 py-2.5 dark:border-gray-800 dark:bg-gray-800/50">
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-1.5">
              <p class="text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">
                {def.name}
                {#if def.required && (!fv || fv.value === null)}
                  <span class="ml-1 text-orange-400">*</span>
                {/if}
              </p>
              {#if def.inherit_from_project}
                <span
                  class="inline-flex items-center gap-0.5 rounded bg-indigo-50 px-1 py-0.5 text-[9px] font-medium text-indigo-500 dark:bg-indigo-950/40 dark:text-indigo-400"
                  title="New assets added to this project will inherit this value"
                >
                  <ArrowDownToLine class="h-2.5 w-2.5" />
                  auto-fills assets
                </span>
              {/if}
            </div>
            {#if didSave}
              <Check class="h-3.5 w-3.5 shrink-0 text-emerald-500" />
            {/if}
          </div>

          {#if isEditing}
            <div class="mt-1.5">
              {#if def.field_type === 'boolean'}
                <div class="flex gap-3">
                  <label class="flex items-center gap-1.5 text-sm text-gray-700 dark:text-gray-300">
                    <input type="radio" bind:group={editValue} value="true" class="text-indigo-600" /> Yes
                  </label>
                  <label class="flex items-center gap-1.5 text-sm text-gray-700 dark:text-gray-300">
                    <input type="radio" bind:group={editValue} value="false" class="text-indigo-600" /> No
                  </label>
                </div>
              {:else if def.field_type === 'select'}
                <select
                  bind:value={editValue}
                  class="w-full rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-sm text-gray-900
                    focus:border-indigo-400 focus:outline-none focus:ring-2 focus:ring-indigo-200
                    dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                >
                  <option value="">— clear —</option>
                  {#each (def.options ? JSON.parse(def.options) as string[] : []) as opt}
                    <option value={opt}>{opt}</option>
                  {/each}
                </select>
              {:else if def.field_type === 'date'}
                <input
                  type="date"
                  bind:value={editValue}
                  onkeydown={(e) => handleKeydown(e, def)}
                  class="w-full rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-sm text-gray-900
                    focus:border-indigo-400 focus:outline-none focus:ring-2 focus:ring-indigo-200
                    dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                />
              {:else if def.field_type === 'number'}
                <input
                  type="number"
                  step="any"
                  bind:value={editValue}
                  onkeydown={(e) => handleKeydown(e, def)}
                  class="w-full rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-sm text-gray-900
                    focus:border-indigo-400 focus:outline-none focus:ring-2 focus:ring-indigo-200
                    dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                />
              {:else if def.field_type === 'url'}
                <input
                  type="url"
                  bind:value={editValue}
                  placeholder="https://"
                  onkeydown={(e) => handleKeydown(e, def)}
                  class="w-full rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-sm text-gray-900
                    focus:border-indigo-400 focus:outline-none focus:ring-2 focus:ring-indigo-200
                    dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                />
              {:else}
                <input
                  type="text"
                  bind:value={editValue}
                  onkeydown={(e) => handleKeydown(e, def)}
                  class="w-full rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-sm text-gray-900
                    focus:border-indigo-400 focus:outline-none focus:ring-2 focus:ring-indigo-200
                    dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                />
              {/if}

              {#if saveError}
                <p class="mt-1 text-xs text-red-600 dark:text-red-400">{saveError}</p>
              {/if}

              <div class="mt-1.5 flex gap-2">
                <button
                  class="text-xs font-medium text-indigo-600 hover:text-indigo-800 dark:text-indigo-400 dark:hover:text-indigo-300 disabled:opacity-50"
                  disabled={isSaving}
                  onclick={() => saveField(def)}
                >
                  {isSaving ? 'Saving…' : 'Save'}
                </button>
                <button
                  class="text-xs text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                  onclick={cancelEdit}
                >
                  Cancel
                </button>
              </div>
            </div>

          {:else if def.field_type === 'boolean' && fv}
            <button
              class="mt-1 flex items-center gap-2 text-sm font-medium
                {fv.value ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-400 dark:text-gray-500'}"
              onclick={() => {
                editValue = fv.value ? 'false' : 'true'
                editingFieldId = def.id
                saveField(def)
              }}
            >
              <span class="h-4 w-7 rounded-full transition-colors {fv.value ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-gray-600'} relative">
                <span class="absolute top-0.5 h-3 w-3 rounded-full bg-white shadow transition-transform {fv.value ? 'left-3.5' : 'left-0.5'}"></span>
              </span>
              {fv.value ? 'Yes' : 'No'}
            </button>

          {:else if fv && fv.value !== null && fv.value !== undefined}
            <button
              class="mt-1 flex w-full items-center justify-between text-left"
              onclick={() => startEdit(def)}
            >
              <span class="text-sm font-semibold text-gray-900 dark:text-gray-100 {def.field_type === 'url' ? 'truncate text-indigo-600 dark:text-indigo-400' : ''}">
                {displayValue(fv)}
              </span>
              <span class="shrink-0 text-xs text-gray-400 opacity-0 transition-opacity hover:opacity-100">Edit</span>
            </button>

          {:else}
            <button
              class="mt-1 text-xs text-gray-400 hover:text-indigo-600 dark:hover:text-indigo-400"
              onclick={() => startEdit(def)}
            >
              Add value
            </button>
          {/if}
        </div>
      {/each}
    </div>
  {/if}

  {#if orphanedValues.length > 0}
    <div class="mt-4">
      <button
        class="flex items-center gap-1 text-xs text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
        onclick={() => { showDeprecated = !showDeprecated }}
      >
        {#if showDeprecated}
          <ChevronDown class="h-3.5 w-3.5" />
        {:else}
          <ChevronRight class="h-3.5 w-3.5" />
        {/if}
        Deprecated fields ({orphanedValues.length})
      </button>

      {#if showDeprecated}
        <div class="mt-2 space-y-1.5">
          {#each orphanedValues as fv}
            <div class="rounded-lg border border-dashed border-gray-200 px-3 py-2 dark:border-gray-700">
              <p class="text-[10px] font-semibold uppercase tracking-widest text-gray-300 dark:text-gray-600">
                {fv.name} <span class="ml-1 normal-case text-gray-300 dark:text-gray-600">(deleted)</span>
              </p>
              <p class="mt-0.5 text-sm text-gray-400 dark:text-gray-500">{displayValue(fv)}</p>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
</div>
