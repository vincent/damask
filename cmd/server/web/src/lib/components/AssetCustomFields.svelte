<script lang="ts">
  import { assetFieldApi, fieldDefinitionApi } from '$lib/api/client'
  import type { Asset, AssetFieldValue, FieldDefinition } from '$lib/api/models'
  import { ChevronDown, ChevronRight, Camera } from '@lucide/svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import SubSectionTitle from './ui/SubSectionTitle.svelte'
  import FieldCard from './FieldCard.svelte'
  import { m } from '$lib/paraglide/messages'

  interface Props {
    asset: Asset
  }

  let { asset }: Props = $props()

  let definitions = $state<FieldDefinition[]>([])
  let values = $state<AssetFieldValue[]>([])
  let loading = $state(true)
  let showDeprecated = $state(false)

  let editingFieldId = $state<string | null>(null)
  let editValue = $state<string>('')
  let savingFieldId = $state<string | null>(null)
  let saveSuccess = $state<string | null>(null)
  let saveError = $state<string | null>(null)

  $effect(() => {
    if (asset.id) load()
  })

  async function load() {
    loading = true
    try {
      const [defs, vals] = await Promise.all([
        fieldDefinitionApi.list('asset'),
        assetFieldApi.get(asset.id),
      ])
      definitions = defs
      values = vals.fields
    } finally {
      loading = false
    }
  }

  function valueFor(fieldId: string): AssetFieldValue | undefined {
    return values.find((v) => v.field_id === fieldId)
  }

  function displayValue(fv: AssetFieldValue): string {
    if (fv.value === null || fv.value === undefined) return ''
    if (fv.field_type === 'boolean') return fv.value ? m.yes() : m.no()
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
      editValue = ''
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

      const result = await assetFieldApi.patch(asset.id, [{ field_id: def.id, value: parsedValue }])
      values = result.fields
      editingFieldId = null
      saveSuccess = def.id
      setTimeout(() => { if (saveSuccess === def.id) saveSuccess = null }, 2000)
    } catch (e: unknown) {
      saveError = e instanceof Error ? e.message : m.save_failed()
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

  function toggleBoolean(def: FieldDefinition) {
    const fv = valueFor(def.id)
    editValue = fv?.value ? 'false' : 'true'
    editingFieldId = def.id
    saveField(def)
  }

  const activeDefinitions = $derived(definitions.filter((d) => !d.deleted_at && !d.key.startsWith('_exif_')))
  const exifDefinitions = $derived(definitions.filter((d) => !d.deleted_at && d.key.startsWith('_exif_')))
  const orphanedValues = $derived(values.filter((v) => v.definition_deleted))
  $inspect({ definitions, values, activeDefinitions, exifDefinitions, orphanedValues }).with(console.log);
</script>

<div>
  <SubSectionTitle>{m.custom_fields_title()}</SubSectionTitle>

  {#if loading}
    <div class="flex justify-center py-4">
      <Spinner size="sm" />
    </div>
  {:else if activeDefinitions.length === 0 && exifDefinitions.length === 0}
    <p class="text-sm text-gray-400 dark:text-gray-500">{m.no_custom_fields_yet()}</p>
  {:else}
    <div class="space-y-2">
      {#each activeDefinitions as def (def.id)}
        <FieldCard
          {def}
          fv={valueFor(def.id)}
          showUnset
          isEditing={editingFieldId === def.id}
          isSaving={savingFieldId === def.id}
          didSave={saveSuccess === def.id}
          {editValue}
          {saveError}
          onStartEdit={() => startEdit(def)}
          onSave={() => saveField(def)}
          onCancel={cancelEdit}
          onKeydown={(e) => handleKeydown(e, def)}
          onToggle={() => toggleBoolean(def)}
          onEditValueChange={(v) => { editValue = v }}
        />
      {/each}
    </div>

    {#if exifDefinitions.length > 0}
      <div class="mt-5">
        <div class="mb-2 flex items-center gap-1.5">
          <Camera class="h-3.5 w-3.5 text-gray-400 dark:text-gray-500" />
          <span class="text-xs font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">EXIF</span>
        </div>
        <div class="space-y-2">
          {#each exifDefinitions as def (def.id)}
            <FieldCard
              {def}
              fv={valueFor(def.id)}
              displayName={def.name.replace(/^_exif_/, '')}
              showUnset
              isEditing={editingFieldId === def.id}
              isSaving={savingFieldId === def.id}
              didSave={saveSuccess === def.id}
              {editValue}
              {saveError}
              onStartEdit={() => startEdit(def)}
              onSave={() => saveField(def)}
              onCancel={cancelEdit}
              onKeydown={(e) => handleKeydown(e, def)}
              onToggle={() => toggleBoolean(def)}
              onEditValueChange={(v) => { editValue = v }}
            />
          {/each}
        </div>
      </div>
    {/if}
  {/if}

  {#if orphanedValues.length > 0}
    <div class="mt-4">
      <button
        class="flex items-center gap-1 text-sm text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
        onclick={() => { showDeprecated = !showDeprecated }}
      >
        {#if showDeprecated}
          <ChevronDown class="h-3.5 w-3.5" />
        {:else}
          <ChevronRight class="h-3.5 w-3.5" />
        {/if}
        {m.fields_deprecated()} ({orphanedValues.length})
      </button>

      {#if showDeprecated}
        <div class="mt-2 space-y-1.5">
          {#each orphanedValues as fv}
            <div class="rounded-lg border border-dashed border-gray-200 px-3 py-2 dark:border-gray-700">
              <p class="text-xs font-semibold uppercase tracking-widest text-gray-300 dark:text-gray-600">
                {fv.name} <span class="ml-1 normal-case text-gray-300 dark:text-gray-600">(deleted)</span>
              </p>
              <p class="mt-0.5 text-md text-gray-400 dark:text-gray-500">{displayValue(fv)}</p>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
</div>
