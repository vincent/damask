<script lang="ts">
  import { fieldDefinitionApi } from '$lib/api/client'
  import type { FieldDefinition, FieldFilter } from '$lib/api/models'
  import { onMount } from 'svelte'
  import Chip from '$lib/components/ui/Chip.svelte'
  import FieldFilterInput from '$lib/components/FieldFilterInput.svelte'
  import { ChevronDown, ChevronRight } from '@lucide/svelte'

  interface Props {
    activeFilters: FieldFilter[]
    onchange: (filters: FieldFilter[]) => void
  }

  let { activeFilters, onchange }: Props = $props()

  let definitions = $state<FieldDefinition[]>([])
  let showExif = $state(false)

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
    clearField(f.key)
  }

  const activeDefinitions = $derived(definitions.filter((d) => !d.deleted_at && !d.key.startsWith('_exif_')))
  const exifDefinitions = $derived(definitions.filter((d) => !d.deleted_at && d.key.startsWith('_exif_')))
  const hasFilters = $derived(activeFilters.length > 0)
</script>

{#if activeDefinitions.length > 0 || exifDefinitions.length > 0}
  <div class="border-t border-gray-100 bg-white dark:border-gray-800 dark:bg-gray-900">

    <!-- Active filter chips -->
    {#if hasFilters}
      <div class="flex flex-wrap items-center gap-1.5 px-6 py-2 border-b border-gray-100 dark:border-gray-800">
        {#each activeFilters as f}
          <Chip label={chipLabel(f)} onremove={() => removeChip(f)} color="#6366f1" />
        {/each}
        <button
          class="ml-1 text-sm text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
          onclick={() => {
            for (const def of definitions) clearField(def.key)
          }}
        >
          Clear all
        </button>
      </div>
    {/if}

    <!-- Filter controls per field -->
    {#if activeDefinitions.length > 0}
      <div class="flex flex-wrap items-end gap-x-6 gap-y-3 px-6 py-3">
        {#each activeDefinitions as def}
          <FieldFilterInput {def} {local} onchange={emit} ondebouncedchange={debouncedEmit} />
        {/each}

        <!-- EXIF fields (collapsed by default) -->
        {#if exifDefinitions.length > 0}
            {#if !showExif}
              <button
                class="flex items-center gap-1 text-sm text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                onclick={() => { showExif = !showExif }}
              >
                EXIF fields
              </button>
            {:else}
                {#each exifDefinitions as def}
                  <FieldFilterInput {def} {local} onchange={emit} ondebouncedchange={debouncedEmit} />
                {/each}
            {/if}
        {/if}

      </div>
    {/if}
  </div>
{/if}
