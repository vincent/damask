<script lang="ts">
  import type {
    WorkflowGraphNode,
    WorkflowNodeConfigSchema,
    WorkflowNodeSchema,
  } from '$lib/api/workflows'
  import Input from '$lib/components/ui/Input.svelte'

  interface Props {
    node: WorkflowGraphNode | null
    schema: WorkflowNodeSchema | null
    onUpdate?: (config: Record<string, unknown>) => void
    onDelete?: () => void
    readonly?: boolean
  }

  let {
    node,
    schema,
    onUpdate = () => {},
    onDelete = () => {},
    readonly = false,
  }: Props = $props()

  const configSchema = $derived(schema?.config_schema ?? null)
  const properties = $derived(
    Object.entries(configSchema?.properties ?? {}) as Array<
      [string, WorkflowNodeConfigSchema]
    >
  )

  function currentConfig() {
    return node?.config ?? {}
  }

  function updateField(key: string, value: unknown) {
    onUpdate({ ...currentConfig(), [key]: value })
  }

  function updateJsonField(key: string, raw: string) {
    if (raw.trim() === '') {
      const next = { ...currentConfig() }
      delete next[key]
      onUpdate(next)
      return
    }
    try {
      updateField(key, JSON.parse(raw))
    } catch {
      updateField(key, raw)
    }
  }

  function stringValue(key: string) {
    const value = currentConfig()[key]
    if (typeof value === 'string') return value
    if (value == null) return ''
    return String(value)
  }

  function jsonValue(key: string) {
    const value = currentConfig()[key]
    if (value == null || value === '') return ''
    if (typeof value === 'string') return value
    return JSON.stringify(value, null, 2)
  }

  function numberValue(key: string) {
    const value = currentConfig()[key]
    if (typeof value === 'number') return value
    if (typeof value === 'string' && value !== '') return Number(value)
    return undefined
  }

  function boolValue(key: string) {
    return Boolean(currentConfig()[key])
  }

  function arrayValue(key: string) {
    const value = currentConfig()[key]
    if (!Array.isArray(value)) return ''
    return value.join('\n')
  }
</script>

<div
  class="flex flex-col rounded-[24px] border border-[var(--border-subtle)] bg-[var(--bg-surface)]"
>
  <div
    class="flex items-start justify-between gap-3 border-b border-[var(--border-subtle)] px-4 py-3.5"
  >
    <div class="min-w-0">
      <p
        class="text-[10px] font-semibold tracking-[0.18em] text-[var(--text-muted)] uppercase"
      >
        Node Config
      </p>
      <h3 class="mt-1 text-sm font-semibold text-[var(--text-primary)]">
        {schema?.label ?? 'No selection'}
      </h3>
      {#if schema?.description}
        <p class="mt-0.5 text-xs leading-relaxed text-[var(--text-secondary)]">
          {schema.description}
        </p>
      {/if}
    </div>

    {#if node && !readonly}
      <button
        type="button"
        class="shrink-0 rounded-lg border border-rose-500/25 px-2.5 py-1 text-[11px] font-semibold text-rose-600 transition-colors hover:bg-rose-500/8 dark:text-rose-400"
        onclick={() => onDelete()}
      >
        Delete
      </button>
    {/if}
  </div>

  <div class="flex-1 p-4">
    {#if !node || !schema}
      <p class="text-xs leading-relaxed text-[var(--text-muted)]">
        Select a node on the canvas to configure it.
      </p>
    {:else if properties.length === 0}
      <p class="text-xs text-[var(--text-muted)]">
        No configuration options for this node.
      </p>
    {:else}
      <div class="space-y-4">
        {#each properties as [key, field] (key)}
          <div class="space-y-1.5">
            {#if field.enum && field.enum.length > 0}
              <label
                class="block text-sm font-medium text-[var(--text-primary)]"
              >
                {field.title ?? key}
              </label>
              <select
                class="w-full rounded-lg border border-[var(--border-default)] bg-[var(--bg-elevated)] px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent)]/20 focus:outline-none"
                disabled={readonly}
                value={stringValue(key)}
                onchange={(event) =>
                  updateField(
                    key,
                    (event.currentTarget as HTMLSelectElement).value
                  )}
              >
                <option value="">Select…</option>
                {#each field.enum as option (option)}
                  <option value={option}>{option}</option>
                {/each}
              </select>
            {:else if field.type === 'boolean'}
              <label
                class="flex cursor-pointer items-center gap-3 text-sm text-[var(--text-primary)]"
              >
                <input
                  type="checkbox"
                  class="h-4 w-4 rounded accent-[var(--accent)]"
                  disabled={readonly}
                  checked={boolValue(key)}
                  onchange={(event) =>
                    updateField(
                      key,
                      (event.currentTarget as HTMLInputElement).checked
                    )}
                />
                {field.title ?? key}
              </label>
            {:else if field.type === 'number'}
              <Input
                label={field.title ?? key}
                type="number"
                disabled={readonly}
                value={numberValue(key)?.toString() ?? ''}
                onchange={(event) => {
                  const raw = (event.currentTarget as HTMLInputElement).value
                  updateField(key, raw === '' ? null : Number(raw))
                }}
              />
            {:else if field.type === 'array' && field.items?.type === 'string'}
              <label
                class="block text-sm font-medium text-[var(--text-primary)]"
              >
                {field.title ?? key}
              </label>
              <textarea
                class="min-h-24 w-full rounded-lg border border-[var(--border-default)] bg-[var(--bg-elevated)] px-3 py-2 text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent)]/20 focus:outline-none"
                disabled={readonly}
                value={arrayValue(key)}
                placeholder="One value per line"
                oninput={(event) =>
                  updateField(
                    key,
                    (event.currentTarget as HTMLTextAreaElement).value
                      .split('\n')
                      .map((item) => item.trim())
                      .filter(Boolean)
                  )}
              ></textarea>
            {:else if field.type === 'object' || field.format === 'json' || field.format === 'template'}
              <label
                class="block text-sm font-medium text-[var(--text-primary)]"
              >
                {field.title ?? key}
              </label>
              <textarea
                class="min-h-28 w-full rounded-lg border border-[var(--border-default)] bg-[var(--bg-elevated)] px-3 py-2 font-mono text-sm text-[var(--text-primary)] focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent)]/20 focus:outline-none"
                disabled={readonly}
                value={jsonValue(key)}
                placeholder={field.format === 'template'
                  ? '{{ ctx.asset_id }}'
                  : '{\n  "key": "value"\n}'}
                oninput={(event) =>
                  updateJsonField(
                    key,
                    (event.currentTarget as HTMLTextAreaElement).value
                  )}
              ></textarea>
            {:else}
              <Input
                label={field.title ?? key}
                disabled={readonly}
                value={stringValue(key)}
                placeholder={field.placeholder}
                onchange={(event) =>
                  updateField(
                    key,
                    (event.currentTarget as HTMLInputElement).value
                  )}
              />
            {/if}

            {#if field.format === 'cron'}
              <p class="text-xs text-[var(--text-muted)]">
                Standard cron expression, e.g. <code class="font-mono"
                  >0 8 * * 1-5</code
                >.
              </p>
            {:else if field.format === 'folder'}
              <p class="text-xs text-[var(--text-muted)]">
                Enter a folder ID — picker coming soon.
              </p>
            {:else if field.format === 'tag'}
              <p class="text-xs text-[var(--text-muted)]">
                Enter a tag name — picker coming soon.
              </p>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>
