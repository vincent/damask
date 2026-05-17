<script lang="ts">
  import { onMount } from 'svelte'
  import { page } from '$app/state'
  import { goto } from '$app/navigation'
  import RunDetail from '$lib/components/workflows/RunDetail.svelte'
  import RunHistory from '$lib/components/workflows/RunHistory.svelte'
  import EmptyState from '$lib/components/ui/EmptyState.svelte'
  import PageHeader from '$lib/components/ui/PageHeader.svelte'
  import {
    workflowsApi,
    type Workflow,
    type WorkflowGraph,
    type WorkflowRun,
    type WorkflowNodeSchema,
  } from '$lib/api/workflows'
  import { toastStore } from '$lib/stores/toast.svelte'

  let loading = $state(true)
  let workflows = $state<Workflow[]>([])
  let nodeSchemas = $state<WorkflowNodeSchema[]>([])

  let selectedWorkflowId = $state<string | null>(null)
  let selectedRunId = $state<string | null>(null)
  let selectedRun = $state<WorkflowRun | null>(null)
  let runsByWorkflow = $state<Record<string, WorkflowRun[]>>({})

  const selectedWorkflow = $derived(
    workflows.find((w) => w.id === selectedWorkflowId) ?? null
  )
  const selectedRuns = $derived(
    selectedWorkflowId ? (runsByWorkflow[selectedWorkflowId] ?? []) : []
  )
  const persistedGraph = $derived(
    selectedWorkflow ? parseGraph(selectedWorkflow.graph) : null
  )

  onMount(() => {
    void load()
  })

  async function load() {
    try {
      const [workflowRows, schemaRows] = await Promise.all([
        workflowsApi.list(),
        workflowsApi.getNodeSchemas(),
      ])
      workflows = workflowRows
      nodeSchemas = schemaRows

      const paramId = page.url.searchParams.get('workflow')
      const initial =
        (paramId && workflowRows.find((w) => w.id === paramId)?.id) ??
        workflowRows[0]?.id ??
        null

      if (initial) await selectWorkflow(initial)
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : 'Failed to load workflows.',
        'error'
      )
    } finally {
      loading = false
    }
  }

  function parseGraph(raw: string): WorkflowGraph {
    try {
      const parsed = JSON.parse(raw) as WorkflowGraph
      return {
        nodes: Array.isArray(parsed.nodes) ? parsed.nodes : [],
        edges: Array.isArray(parsed.edges) ? parsed.edges : [],
      }
    } catch {
      return { nodes: [], edges: [] }
    }
  }

  function cloneGraph(g: WorkflowGraph): WorkflowGraph {
    return JSON.parse(JSON.stringify(g)) as WorkflowGraph
  }

  async function loadRuns(workflowId: string) {
    const runs = await workflowsApi.listRuns(workflowId)
    runsByWorkflow = { ...runsByWorkflow, [workflowId]: runs }
    return runs
  }

  async function loadRunDetail(workflowId: string, runId: string) {
    selectedRun = await workflowsApi.getRun(workflowId, runId)
    selectedRunId = runId
  }

  async function selectWorkflow(workflowId: string) {
    selectedWorkflowId = workflowId
    selectedRun = null
    selectedRunId = null
    void goto(`?workflow=${workflowId}`, { replaceState: true, noScroll: true })

    try {
      const runs = await loadRuns(workflowId)
      if (runs.length > 0) {
        await loadRunDetail(workflowId, runs[0].id)
      }
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : 'Failed to load runs.',
        'error'
      )
    }
  }

  async function refreshSelectedRun() {
    if (!selectedWorkflowId || !selectedRunId) return
    try {
      await Promise.all([
        loadRunDetail(selectedWorkflowId, selectedRunId),
        loadRuns(selectedWorkflowId),
      ])
    } catch {
      // Ignore transient SSE refresh failures.
    }
  }

  function relativeTime(iso: string): string {
    const diff = Date.now() - new Date(iso).getTime()
    const mins = Math.floor(diff / 60_000)
    if (mins < 1) return 'just now'
    if (mins < 60) return `${mins}m ago`
    const hrs = Math.floor(mins / 60)
    if (hrs < 24) return `${hrs}h ago`
    const days = Math.floor(hrs / 24)
    if (days < 7) return `${days}d ago`
    return new Date(iso).toLocaleDateString()
  }
</script>

<svelte:head>
  <title>Automations — Damask</title>
</svelte:head>

<div class="flex h-full flex-1 flex-col overflow-hidden">
  <PageHeader
    title="Last Automations"
    description="Recent workflow executions, step-by-step."
  />
  <div class="flex min-h-0 flex-1 overflow-hidden">
    <!-- Workflow picker sidebar -->
    <aside
      class="flex w-[260px] shrink-0 flex-col overflow-y-auto border-r border-[var(--border-subtle)]"
    >
      {#if loading}
        <div class="flex flex-col gap-2 p-3">
          {#each [0, 1, 2] as i (i)}
            <div
              class="h-14 animate-pulse rounded-xl bg-[var(--bg-elevated)]"
            ></div>
          {/each}
        </div>
      {:else if workflows.length === 0}
        <div class="px-4 py-6 text-xs text-[var(--text-muted)]">
          No workflows yet.
        </div>
      {:else}
        <nav class="flex flex-col gap-1 p-2">
          {#each workflows as workflow (workflow.id)}
            <button
              type="button"
              data-trigger-type={workflow.trigger_type}
              class="rounded-xl border px-3 py-2.5 text-left transition-colors {selectedWorkflowId ===
              workflow.id
                ? 'border-[var(--accent)]/40 bg-[var(--accent-soft)]'
                : 'border-transparent hover:border-[var(--border-subtle)] hover:bg-[var(--bg-hover)]'}"
              onclick={() => void selectWorkflow(workflow.id)}
            >
              <div class="flex items-center justify-between gap-2">
                <p
                  class="truncate text-[13px] font-semibold {selectedWorkflowId ===
                  workflow.id
                    ? 'text-[var(--accent-text)]'
                    : 'text-[var(--text-primary)]'}"
                >
                  {workflow.name}
                </p>
                <span
                  class="shrink-0 rounded-full px-1.5 py-0.5 text-[10px] font-semibold uppercase {workflow.enabled
                    ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
                    : 'bg-zinc-500/10 text-zinc-500 dark:text-zinc-400'}"
                >
                  {workflow.enabled ? 'On' : 'Off'}
                </span>
              </div>
              {#if workflow.last_run_at}
                <p class="mt-1 text-[11px] text-[var(--text-muted)]">
                  Last run {relativeTime(workflow.last_run_at)}
                </p>
              {:else}
                <p class="mt-1 text-[11px] text-[var(--text-muted)]">
                  Never run
                </p>
              {/if}
            </button>
          {/each}
        </nav>
      {/if}
    </aside>

    <!-- Main content -->
    <div class="flex min-w-0 flex-1 flex-col overflow-hidden">
      {#if !selectedWorkflow}
        <div class="flex flex-1 items-center justify-center px-8">
          <EmptyState
            title="No workflow selected"
            description="Choose a workflow from the sidebar to view its run history."
          />
        </div>
      {:else}
        <div class="flex h-full overflow-hidden">
          <!-- Run history column -->
          <div
            class="w-[280px] shrink-0 overflow-y-auto border-r border-[var(--border-subtle)]"
          >
            <div class="p-4">
              <div class="mb-3">
                <h3 class="text-sm font-semibold text-[var(--text-primary)]">
                  {selectedWorkflow.name}
                </h3>
                <p class="mt-0.5 text-xs text-[var(--text-muted)]">
                  Run history
                </p>
              </div>
              <RunHistory
                runs={selectedRuns}
                {selectedRunId}
                onSelect={(run) => {
                  if (selectedWorkflowId)
                    void loadRunDetail(selectedWorkflowId, run.id)
                }}
              />
            </div>
          </div>

          <!-- Run detail -->
          <div class="min-w-0 flex-1 overflow-y-auto">
            <div class="p-6">
              {#if !selectedRun}
                <div
                  class="rounded-[24px] border border-dashed border-[var(--border-subtle)] bg-[var(--bg-surface)] px-6"
                >
                  <EmptyState
                    title="Select a run"
                    description="Choose a run from the history panel to inspect step-by-step execution."
                  />
                </div>
              {:else}
                <RunDetail
                  run={selectedRun}
                  graph={persistedGraph ? cloneGraph(persistedGraph) : null}
                  schemas={nodeSchemas}
                  onRefresh={refreshSelectedRun}
                />
              {/if}
            </div>
          </div>
        </div>
      {/if}
    </div>
  </div>
</div>
