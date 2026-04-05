<script lang="ts">
  import { fieldDefinitionApi } from '$lib/api/client'
  import type { FieldDefinition, FieldFilter } from '$lib/api/models'
  import { onMount } from 'svelte'
  import Chip from '$lib/components/ui/Chip.svelte'

  interface Props {
    activeFilters: FieldFilter[]
    onchange: (filters: FieldFilter[]) => void
  }

  let { activeFilters, onchange }: Props = $props()

  let definitions = $state<FieldDefinition[]>([])

  onMount(async () => {
    try {
      definitions = await fieldDefinitionApi.list('asset')
    } catch {
      // silently ignore
    }
  })

  // Per-field input state keyed by field.key.
  // text/url: string, number: { min, max }, date: { from, to }, boolean: '' | 'true' | 'false', select: string[]
  type LocalState = Record<string, unknown>
  let local = $state<LocalState>({})

  $effect(() => {
    // When definitions load, seed any new keys without touching existing ones (preserves user input).
    for (const def of definitions) {
      if (def.key in local) continue
      switch (def.field_type) {
        case 'number': local[def.key] = { min: '', max: '' }; break
        case 'date':   local[def.key] = { from: '', to: '' }; break
        default:       local[def.key] = ''; break
      }
    }
  })

  // Rebuild filters from local state and emit
  function emit() {
    const filters: FieldFilter[] = []
    for (const def of definitions) {
      const v = local[def.key]
      if (v === undefined || v === null) continue
      switch (def.field_type) {
        case 'text':
        case 'url': {
          const s = (v as string).trim()
          if (s) filters.push({ key: def.key, op: 'contains', value: s })
          break
        }
        case 'number': {
          const { min, max } = v as { min: string; max: string }
          if (min.trim()) filters.push({ key: def.key, op: 'gte', value: min.trim() })
          if (max.trim()) filters.push({ key: def.key, op: 'lte', value: max.trim() })
          break
        }
        case 'date': {
          const { from, to } = v as { from: string; to: string }
          if (from) filters.push({ key: def.key, op: 'gte', value: from })
          if (to) filters.push({ key: def.key, op: 'lte', value: to })
          break
        }
        case 'boolean': {
          const s = v as string
          if (s === 'true') filters.push({ key: def.key, op: 'eq', value: 'true' })
          else if (s === 'false') filters.push({ key: def.key, op: 'eq', value: 'false' })
          break
        }
        case 'select': {
          // Each selected option emits a separate eq filter — backend handles AND per field
          // but for OR-within-field, we send only the first selected (backend doesn't yet support OR)
          // Per roadmap: "multi-select → OR logic" is noted but the backend supports one eq per key.
          // We'll emit the first selection here and iterate options as radio-style.
          const s = v as string
          if (s) filters.push({ key: def.key, op: 'eq', value: s })
          break
        }
      }
    }
    onchange(filters)
  }

  let debounceTimer: ReturnType<typeof setTimeout>
  function debouncedEmit() {
    clearTimeout(debounceTimer)
    debounceTimer = setTimeout(emit, 350)
  }

  function clearField(key: string) {
    const def = definitions.find((d) => d.key === key)
    if (!def) return
    switch (def.field_type) {
      case 'text': case 'url': local[key] = ''; break
      case 'number': local[key] = { min: '', max: '' }; break
      case 'date': local[key] = { from: '', to: '' }; break
      case 'boolean': local[key] = ''; break
      case 'select': local[key] = ''; break
    }
    emit()
  }

  // Chip label for an active filter
  function chipLabel(f: FieldFilter): string {
    const def = definitions.find((d) => d.key === f.key)
    const name = def?.name ?? f.key
    const opLabels: Record<string, string> = { eq: '=', lt: '<', lte: '≤', gt: '>', gte: '≥', contains: '~', starts_with: '^' }
    return `${name} ${opLabels[f.op] ?? f.op} ${f.value}`
  }

  function removeChip(f: FieldFilter) {
    // Clear the local state for this field's key and re-emit
    clearField(f.key)
  }

  const activeDefinitions = $derived(definitions.filter((d) => !d.deleted_at))
  const hasFilters = $derived(activeFilters.length > 0)
</script>

{#if activeDefinitions.length > 0}
  <div class="border-t border-gray-100 bg-white dark:border-gray-800 dark:bg-gray-900">

    <!-- Active filter chips -->
    {#if hasFilters}
      <div class="flex flex-wrap items-center gap-1.5 px-6 py-2 border-b border-gray-100 dark:border-gray-800">
        {#each activeFilters as f}
          <Chip label={chipLabel(f)} onremove={() => removeChip(f)} color="#6366f1" />
        {/each}
        <button
          class="ml-1 text-xs text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
          onclick={() => {
            for (const def of definitions) clearField(def.key)
          }}
        >
          Clear all
        </button>
      </div>
    {/if}

    <!-- Filter controls per field -->
    <div class="flex flex-wrap items-end gap-x-6 gap-y-3 px-6 py-3">
      {#each activeDefinitions as def}
        <div class="flex flex-col gap-1 min-w-0">
          <label for="field-name" class="text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500">
            {def.name}
          </label>

          {#if def.field_type === 'text' || def.field_type === 'url'}
            <input
              type="text"
              placeholder="contains…"
              value={local[def.key] as string}
              oninput={(e) => { local[def.key] = (e.target as HTMLInputElement).value; debouncedEmit() }}
              class="w-36 rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-xs text-gray-900
                focus:border-indigo-400 focus:outline-none
                dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
            />

          {:else if def.field_type === 'number'}
            {@const nums = local[def.key] as { min: string; max: string }}
            <div class="flex items-center gap-1">
              <input
                type="number"
                step="any"
                placeholder="min"
                value={nums.min}
                oninput={(e) => { (local[def.key] as { min: string; max: string }).min = (e.target as HTMLInputElement).value; debouncedEmit() }}
                class="w-20 rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-xs text-gray-900
                  focus:border-indigo-400 focus:outline-none
                  dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
              />
              <span class="text-xs text-gray-400">–</span>
              <input
                type="number"
                step="any"
                placeholder="max"
                value={nums.max}
                oninput={(e) => { (local[def.key] as { min: string; max: string }).max = (e.target as HTMLInputElement).value; debouncedEmit() }}
                class="w-20 rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-xs text-gray-900
                  focus:border-indigo-400 focus:outline-none
                  dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
              />
            </div>

          {:else if def.field_type === 'date'}
            {@const dates = local[def.key] as { from: string; to: string }}
            <div class="flex items-center gap-1">
              <input
                type="date"
                value={dates.from}
                onchange={(e) => { (local[def.key] as { from: string; to: string }).from = (e.target as HTMLInputElement).value; emit() }}
                class="rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-xs text-gray-900
                  focus:border-indigo-400 focus:outline-none
                  dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
              />
              <span class="text-xs text-gray-400">–</span>
              <input
                type="date"
                value={dates.to}
                onchange={(e) => { (local[def.key] as { from: string; to: string }).to = (e.target as HTMLInputElement).value; emit() }}
                class="rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-xs text-gray-900
                  focus:border-indigo-400 focus:outline-none
                  dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
              />
            </div>

          {:else if def.field_type === 'boolean'}
            {@const bv = local[def.key] as string}
            <div class="flex gap-2">
              {#each [['', 'Any'], ['true', 'Yes'], ['false', 'No']] as [val, label]}
                <button
                  type="button"
                  class="rounded-full border px-2.5 py-1 text-xs transition-colors
                    {bv === val
                      ? 'border-indigo-500 bg-indigo-50 text-indigo-700 dark:border-indigo-400 dark:bg-indigo-950/40 dark:text-indigo-300'
                      : 'border-gray-200 text-gray-600 hover:border-gray-300 dark:border-gray-700 dark:text-gray-400'}"
                  onclick={() => { local[def.key] = val; emit() }}
                >
                  {label}
                </button>
              {/each}
            </div>

          {:else if def.field_type === 'select'}
            {@const opts = def.options ? JSON.parse(def.options) as string[] : []}
            {@const sv = local[def.key] as string}
            <div class="flex flex-wrap gap-1">
              {#each opts as opt}
                <button
                  type="button"
                  class="rounded-full border px-2.5 py-1 text-xs transition-colors
                    {sv === opt
                      ? 'border-indigo-500 bg-indigo-50 text-indigo-700 dark:border-indigo-400 dark:bg-indigo-950/40 dark:text-indigo-300'
                      : 'border-gray-200 text-gray-600 hover:border-gray-300 dark:border-gray-700 dark:text-gray-400'}"
                  onclick={() => { local[def.key] = sv === opt ? '' : opt; emit() }}
                >
                  {opt}
                </button>
              {/each}
            </div>
          {/if}
        </div>
      {/each}
    </div>
  </div>
{/if}
