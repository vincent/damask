<script lang="ts">
  import type { FieldDefinition } from '$lib/api/models'
  import { Check, ArrowDownToLine } from '@lucide/svelte'

  type FieldValue = {
    field_id: string
    field_type: string
    value: string | number | boolean | null
    definition_deleted: boolean
  }

  interface Props {
    def: FieldDefinition
    fv: FieldValue | undefined
    displayName?: string
    showUnset?: boolean
    isEditing: boolean
    isSaving: boolean
    didSave: boolean
    editValue: string
    saveError: string | null
    onStartEdit: () => void
    onSave: () => void
    onCancel: () => void
    onKeydown: (e: KeyboardEvent) => void
    onToggle: () => void
    onEditValueChange: (v: string) => void
  }

  let {
    def,
    fv,
    displayName,
    showUnset = false,
    isEditing,
    isSaving,
    didSave,
    editValue,
    saveError,
    onStartEdit,
    onSave,
    onCancel,
    onKeydown,
    onToggle,
    onEditValueChange,
  }: Props = $props()

  const label = $derived(displayName ?? def.name)

  function displayValue(fv: FieldValue): string {
    if (fv.value === null || fv.value === undefined) return ''
    if (fv.field_type === 'boolean') return fv.value ? 'Yes' : 'No'
    return String(fv.value)
  }
</script>

<div class="rounded-xl border border-gray-100 bg-gray-50 px-3 py-2.5 dark:border-gray-800 dark:bg-gray-800/50">
  <div class="flex items-center justify-between gap-2">
    <div class="flex items-center gap-1.5">
      <p class="text-xs font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">
        {label}
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
          <label class="flex items-center gap-1.5 text-md text-gray-700 dark:text-gray-300">
            <input type="radio" checked={editValue === 'true'} onchange={() => onEditValueChange('true')} class="text-indigo-600" /> Yes
          </label>
          <label class="flex items-center gap-1.5 text-md text-gray-700 dark:text-gray-300">
            <input type="radio" checked={editValue === 'false'} onchange={() => onEditValueChange('false')} class="text-indigo-600" /> No
          </label>
          {#if showUnset}
            <label class="flex items-center gap-1.5 text-md text-gray-700 dark:text-gray-300">
              <input type="radio" checked={editValue === ''} onchange={() => onEditValueChange('')} disabled class="text-indigo-600" /> Unset
            </label>
          {/if}
        </div>
      {:else if def.field_type === 'select'}
        <select
          value={editValue}
          onchange={(e) => onEditValueChange((e.target as HTMLSelectElement).value)}
          class="w-full rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-md text-gray-900
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
          value={editValue}
          oninput={(e) => onEditValueChange((e.target as HTMLInputElement).value)}
          onkeydown={onKeydown}
          class="w-full rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-md text-gray-900
            focus:border-indigo-400 focus:outline-none focus:ring-2 focus:ring-indigo-200
            dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
        />
      {:else if def.field_type === 'number'}
        <input
          type="number"
          step="any"
          value={editValue}
          oninput={(e) => onEditValueChange((e.target as HTMLInputElement).value)}
          onkeydown={onKeydown}
          class="w-full rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-md text-gray-900
            focus:border-indigo-400 focus:outline-none focus:ring-2 focus:ring-indigo-200
            dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
        />
      {:else if def.field_type === 'url'}
        <input
          type="url"
          value={editValue}
          placeholder="https://"
          oninput={(e) => onEditValueChange((e.target as HTMLInputElement).value)}
          onkeydown={onKeydown}
          class="w-full rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-md text-gray-900
            focus:border-indigo-400 focus:outline-none focus:ring-2 focus:ring-indigo-200
            dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
        />
      {:else}
        <input
          type="text"
          value={editValue}
          oninput={(e) => onEditValueChange((e.target as HTMLInputElement).value)}
          onkeydown={onKeydown}
          class="w-full rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-md text-gray-900
            focus:border-indigo-400 focus:outline-none focus:ring-2 focus:ring-indigo-200
            dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
        />
      {/if}

      {#if saveError}
        <p class="mt-1 text-sm text-red-600 dark:text-red-400">{saveError}</p>
      {/if}

      <div class="mt-1.5 flex gap-2">
        <button
          class="text-sm font-medium text-indigo-600 hover:text-indigo-800 dark:text-indigo-400 dark:hover:text-indigo-300 disabled:opacity-50"
          disabled={isSaving}
          onclick={onSave}
        >
          {isSaving ? 'Saving…' : 'Save'}
        </button>
        <button
          class="text-sm text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
          onclick={onCancel}
        >
          Cancel
        </button>
      </div>
    </div>

  {:else if def.field_type === 'boolean' && fv}
    <button
      class="mt-1 flex items-center gap-2 text-md font-medium
        {fv.value ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-400 dark:text-gray-500'}"
      onclick={onToggle}
    >
      <span class="h-4 w-7 rounded-full transition-colors {fv.value ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-gray-600'} relative">
        <span class="absolute top-0.5 h-3 w-3 rounded-full bg-white shadow transition-transform {fv.value ? 'left-3.5' : 'left-0.5'}"></span>
      </span>
      {fv.value ? 'Yes' : 'No'}
    </button>

  {:else if fv && fv.value !== null && fv.value !== undefined}
    <button
      class="mt-1 flex w-full items-center justify-between text-left"
      onclick={onStartEdit}
    >
      <span class="text-md font-semibold text-gray-900 dark:text-gray-100 {def.field_type === 'url' ? 'truncate text-indigo-600 dark:text-indigo-400' : ''}">
        {displayValue(fv)}
      </span>
      <span class="shrink-0 text-sm text-gray-400 opacity-0 transition-opacity hover:opacity-100">Edit</span>
    </button>

  {:else}
    <button
      class="mt-1 text-sm text-gray-400 hover:text-indigo-600 dark:hover:text-indigo-400"
      onclick={onStartEdit}
    >
      Add value
    </button>
  {/if}
</div>
