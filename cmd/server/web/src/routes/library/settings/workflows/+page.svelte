<script lang="ts">
  import { onMount } from 'svelte'
  import {
    ArrowLeft,
    Plus,
    ChevronDown,
    Sparkles,
    Save,
    Pause,
    Play,
    Trash2,
    Zap,
    SquarePen,
  } from '@lucide/svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import EmptyState from '$lib/components/ui/EmptyState.svelte'
  import PageHeader from '$lib/components/ui/PageHeader.svelte'
  import GridSkeleton from '$lib/components/ui/GridSkeleton.svelte'
  import ConfirmModal from '$lib/components/ui/ConfirmModal.svelte'
  import WorkflowCanvas from '$lib/components/workflows/WorkflowCanvas.svelte'
  import WorkflowCard from '$lib/components/workflows/WorkflowCard.svelte'
  import WorkflowRunsPanel from '$lib/components/workflows/WorkflowRunsPanel.svelte'
  import {
    workflowsApi,
    type Workflow,
    type WorkflowGraph,
    type WorkflowTemplate,
    type WorkflowNodeSchema,
    type WorkflowRun,
  } from '$lib/api/workflows'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { workflowsStore } from '$lib/stores/workflows.svelte'
  import RunDetail from '$lib/components/workflows/RunDetail.svelte'

  type View = 'list' | 'editor' | 'inspect'

  let view = $state<View>('list')
  let loading = $state(true)
  let creating = $state(false)
  let saving = $state(false)
  let toggling = $state(false)
  let deleting = $state(false)
  let templates = $state<WorkflowTemplate[]>([])
  let nodeSchemas = $state<WorkflowNodeSchema[]>([])
  let addMenuOpen = $state(false)
  let addMenuEl = $state<HTMLDivElement | undefined>()

  $effect(() => {
    if (!addMenuOpen) return
    function handleClickOutside(e: MouseEvent) {
      if (addMenuEl && !addMenuEl.contains(e.target as Node)) {
        addMenuOpen = false
      }
    }
    document.addEventListener('click', handleClickOutside, { capture: true })
    return () =>
      document.removeEventListener('click', handleClickOutside, {
        capture: true,
      })
  })

  let selectedWorkflow = $state<Workflow | null>(null)
  let panelWorkflow = $state<Workflow | null>(null)
  let inspectRun = $state<WorkflowRun | null>(null)

  let pendingDeleteWorkflow = $state<Workflow | null>(null)
  let showDeleteConfirm = $state(false)

  // Editor draft state
  let draftName = $state('')
  let draftDescription = $state('')
  let draftNotifyEmail = $state('')
  let editableGraph = $state<WorkflowGraph>({ nodes: [], edges: [] })
  let baseline = $state({
    name: '',
    description: '',
    notify: '',
    graph: '{"nodes":[],"edges":[]}',
  })

  const dirty = $derived(
    selectedWorkflow !== null &&
      (draftName !== baseline.name ||
        draftDescription !== baseline.description ||
        draftNotifyEmail !== baseline.notify ||
        JSON.stringify(editableGraph) !== baseline.graph)
  )

  onMount(() => {
    void load()
  })

  async function load() {
    loading = true
    try {
      const [templateRows, schemaRows] = await Promise.all([
        workflowsApi.getTemplates(),
        workflowsApi.getNodeSchemas(),
      ])
      templates = templateRows
      nodeSchemas = schemaRows
      await workflowsStore.load()
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

  function cloneGraph(graph: WorkflowGraph): WorkflowGraph {
    return JSON.parse(JSON.stringify(graph)) as WorkflowGraph
  }

  function setEditorState(workflow: Workflow) {
    const graph = parseGraph(workflow.graph)
    draftName = workflow.name
    draftDescription = workflow.description
    draftNotifyEmail = workflow.notify_on_failure_email ?? ''
    editableGraph = cloneGraph(graph)
    baseline = {
      name: workflow.name,
      description: workflow.description,
      notify: workflow.notify_on_failure_email ?? '',
      graph: JSON.stringify(graph),
    }
  }

  function openEditor(workflow: Workflow) {
    selectedWorkflow = workflow
    workflowsStore.selectedId = workflow.id
    setEditorState(workflow)
    inspectRun = null
    view = 'editor'
  }

  async function openInspect(workflow: Workflow, run: WorkflowRun) {
    selectedWorkflow = workflow
    workflowsStore.selectedId = workflow.id
    setEditorState(workflow)
    inspectRun = run
    view = 'inspect'
    try {
      inspectRun = await workflowsApi.getRun(workflow.id, run.id)
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : 'Failed to load run details.',
        'error'
      )
    }
  }

  function goToList() {
    view = 'list'
    inspectRun = null
  }

  function handleCardClick(workflow: Workflow) {
    if (panelWorkflow?.id === workflow.id) {
      panelWorkflow = null
    } else {
      panelWorkflow = workflow
    }
  }

  async function createFromTemplate(template: WorkflowTemplate) {
    addMenuOpen = false
    creating = true
    try {
      const workflow = await workflowsApi.create({
        name: template.name,
        description: template.description,
        graph: template.graph,
      })
      workflowsStore.upsert(workflow)
      panelWorkflow = workflow
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : 'Failed to create workflow.',
        'error'
      )
    } finally {
      creating = false
    }
  }

  async function saveWorkflow() {
    if (!selectedWorkflow) return
    saving = true
    try {
      const workflow = await workflowsApi.update(selectedWorkflow.id, {
        name: draftName,
        description: draftDescription,
        graph: JSON.stringify(editableGraph),
        notify_on_failure_email: draftNotifyEmail,
      })
      workflowsStore.upsert(workflow)
      selectedWorkflow = workflow
      setEditorState(workflow)
      toastStore.show(`Workflow "${workflow.name}" saved.`)
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : 'Failed to save workflow.',
        'error'
      )
    } finally {
      saving = false
    }
  }

  async function toggleWorkflow(workflow: Workflow) {
    toggling = true
    try {
      await workflowsApi.setEnabled(workflow.id, !workflow.enabled)
      workflowsStore.patch(workflow.id, { enabled: !workflow.enabled })
      if (selectedWorkflow?.id === workflow.id) {
        selectedWorkflow = { ...workflow, enabled: !workflow.enabled }
      }
      if (panelWorkflow?.id === workflow.id) {
        panelWorkflow = { ...workflow, enabled: !workflow.enabled }
      }
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : 'Failed to update workflow.',
        'error'
      )
    } finally {
      toggling = false
    }
  }

  async function runWorkflow(workflow: Workflow) {
    try {
      await workflowsApi.triggerManual(workflow.id)
      toastStore.show(`Workflow "${workflow.name}" triggered.`)
      if (panelWorkflow?.id === workflow.id) {
        panelWorkflow = { ...panelWorkflow }
      }
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : 'Failed to trigger workflow.',
        'error'
      )
    }
  }

  function handleDelete(workflow: Workflow) {
    pendingDeleteWorkflow = workflow
    showDeleteConfirm = true
  }

  async function confirmDelete() {
    if (!pendingDeleteWorkflow) return
    const wf = pendingDeleteWorkflow
    pendingDeleteWorkflow = null
    deleting = true
    try {
      await workflowsApi.delete(wf.id)
      workflowsStore.remove(wf.id)
      if (panelWorkflow?.id === wf.id) panelWorkflow = null
      if (selectedWorkflow?.id === wf.id) {
        selectedWorkflow = null
        view = 'list'
      }
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : 'Failed to delete workflow.',
        'error'
      )
    } finally {
      deleting = false
    }
  }
</script>

<svelte:head>
  <title>Workflows — Damask</title>
</svelte:head>

<div class="flex flex-1 flex-col overflow-hidden">
  {#if view === 'list'}
    <!-- ── LIST VIEW ─────────────────────────────────────────── -->
    <PageHeader
      title="Workflows"
      description="Automate DAM actions with trigger, filter, action, and control-flow graphs."
    >
      <div class="wf-add-group relative flex" bind:this={addMenuEl}>
        <button
          type="button"
          disabled={creating}
          class="wf-add-primary"
          onclick={() => (addMenuOpen = !addMenuOpen)}
          aria-haspopup="menu"
          aria-expanded={addMenuOpen}
        >
          <Plus class="h-4 w-4" />
          Add workflow
        </button>
        <button
          type="button"
          disabled={creating}
          class="wf-add-chevron"
          onclick={() => (addMenuOpen = !addMenuOpen)}
          aria-label="Choose workflow template"
          aria-haspopup="menu"
          aria-expanded={addMenuOpen}
        >
          <ChevronDown
            class="h-4 w-4 transition-transform duration-150 {addMenuOpen
              ? 'rotate-180'
              : ''}"
          />
        </button>

        {#if addMenuOpen}
          <div
            role="menu"
            tabindex="-1"
            class="wf-add-dropdown"
            onkeydown={(e) => e.key === 'Escape' && (addMenuOpen = false)}
          >
            <div class="add-dropdown-divider"></div>

            {#each templates as template (template.id)}
              <button
                type="button"
                role="menuitem"
                class="wf-add-item"
                onclick={() => void createFromTemplate(template)}
              >
                <Sparkles class="wf-add-item-icon" />
                <div class="min-w-0">
                  <p class="text-sm font-medium text-[var(--text-primary)]">
                    {template.name}
                  </p>
                  <p
                    class="mt-0.5 text-xs leading-relaxed text-[var(--text-secondary)]"
                  >
                    {template.description}
                  </p>
                </div>
              </button>
            {/each}
          </div>
        {/if}
      </div>
    </PageHeader>

    <div class="flex grow overflow-y-auto">
      <!-- Workflow list -->
      <main class="mx-auto mb-20 flex-1 px-6 py-6">
        {#if loading}
          <GridSkeleton lines={3} />
        {:else if workflowsStore.workflows.length === 0}
          <EmptyState
            title="No workflows yet"
            description={`Use "Add workflow" to create your first automation.`}
          >
            {#snippet icon()}
              <Zap class="h-10 w-10" />
            {/snippet}
          </EmptyState>
        {:else}
          <div class="space-y-3">
            {#each workflowsStore.workflows as workflow (workflow.id)}
              <WorkflowCard
                {workflow}
                selected={panelWorkflow?.id === workflow.id}
                onrun={runWorkflow}
                ontoggle={toggleWorkflow}
                onedit={openEditor}
                ondelete={handleDelete}
                onclick={handleCardClick}
              />
            {/each}
          </div>
        {/if}
      </main>

      <!-- Runs panel -->
      {#if panelWorkflow}
        <WorkflowRunsPanel
          workflow={panelWorkflow}
          onclose={() => (panelWorkflow = null)}
          oninspect={(run) => {
            if (panelWorkflow) void openInspect(panelWorkflow, run)
          }}
        />
      {/if}
    </div>
  {:else}
    <!-- ── EDITOR / INSPECT VIEW ──────────────────────────────── -->
    <PageHeader
      title={view === 'inspect'
        ? 'Inspect run'
        : (selectedWorkflow?.name ?? 'Editor')}
      description={view === 'inspect'
        ? undefined
        : 'Edit workflow graph and settings.'}
    >
      {#snippet meta()}
        {#if view === 'inspect' && inspectRun}
          <div class="flex flex-wrap items-center gap-x-4 gap-y-1">
            <span class="font-mono text-[11px] text-[var(--text-muted)]"
              >Run {inspectRun.id.slice(0, 8)}</span
            >
            <span
              class="inline-flex rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase {inspectRun.status ===
              'completed'
                ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
                : inspectRun.status === 'failed'
                  ? 'bg-rose-500/10 text-rose-700 dark:text-rose-300'
                  : inspectRun.status === 'running'
                    ? 'bg-sky-500/10 text-sky-700 dark:text-sky-300'
                    : 'bg-slate-500/10 text-slate-700 dark:text-slate-300'}"
            >
              {inspectRun.status}
            </span>
            {#if inspectRun.started_at}
              <span class="text-[11px] text-[var(--text-muted)]">
                Started <span class="text-[var(--text-secondary)]"
                  >{new Date(inspectRun.started_at).toLocaleString()}</span
                >
              </span>
            {/if}
            {#if inspectRun.completed_at}
              <span class="text-[11px] text-[var(--text-muted)]">
                Completed <span class="text-[var(--text-secondary)]"
                  >{new Date(inspectRun.completed_at).toLocaleString()}</span
                >
              </span>
            {/if}
          </div>
        {/if}
        {#if view === 'editor' && dirty !== undefined}
          <div
            class="mt-0.5 inline-flex items-center gap-1.5 text-[11px] {dirty
              ? 'text-amber-500'
              : 'text-transparent'}"
          >
            <span class="h-1.5 w-1.5 rounded-full {dirty ? 'bg-amber-400' : ''}"
            ></span>
            {dirty ? 'Unsaved changes' : '·'}
          </div>
        {/if}
      {/snippet}
      <div class="flex items-center gap-2">
        <Button variant="ghost" onclick={goToList}>
          {#snippet icon()}<ArrowLeft class="h-4 w-4" />{/snippet}
          Back
        </Button>

        {#if view === 'inspect' && selectedWorkflow}
          <Button
            variant="ghost"
            onclick={() => selectedWorkflow && openEditor(selectedWorkflow)}
          >
            {#snippet icon()}<SquarePen class="h-4 w-4" />{/snippet}
            Edit workflow
          </Button>
        {/if}

        {#if view === 'editor' && selectedWorkflow}
          <Button
            variant="ghost"
            disabled={toggling}
            onclick={() =>
              selectedWorkflow && void toggleWorkflow(selectedWorkflow)}
          >
            {#if selectedWorkflow.enabled}
              <Pause class="h-4 w-4" />
              Pause
            {:else}
              <Play class="h-4 w-4" />
              Enable
            {/if}
          </Button>
          <Button
            variant="ghost"
            disabled={deleting}
            onclick={() => selectedWorkflow && handleDelete(selectedWorkflow)}
            class="text-red-500 hover:text-red-600"
          >
            <Trash2 class="h-4 w-4" />
            Delete
          </Button>
          <Button
            disabled={!dirty || saving}
            onclick={() => void saveWorkflow()}
          >
            <Save class="h-4 w-4" />
            {saving ? 'Saving…' : 'Save'}
          </Button>
        {/if}
      </div>
    </PageHeader>

    {#if view === 'inspect' && inspectRun && selectedWorkflow}
      <div class="flex min-h-0 flex-1 flex-col gap-6 overflow-y-auto p-6">
        <!-- Inspect: readonly canvas + run detail -->
        <RunDetail
          run={inspectRun}
          graph={parseGraph(selectedWorkflow.graph)}
          schemas={nodeSchemas}
        />
      </div>
    {/if}
    {#if view === 'editor' && selectedWorkflow}
      <div class="flex min-h-0 flex-1 flex-col gap-6">
        <!-- Editor: name/desc/email + canvas -->
        <div
          class="border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-5 pt-0"
        >
          <div class="grid gap-4 lg:grid-cols-3">
            <Input label="Name" bind:value={draftName} />
            <Input label="Description" bind:value={draftDescription} />
            <Input
              label="Failure Email"
              bind:value={draftNotifyEmail}
              placeholder="alerts@example.com"
            />
          </div>
        </div>

        <div class="mb-10 overflow-y-auto">
          <WorkflowCanvas bind:graph={editableGraph} schemas={nodeSchemas} />
        </div>
      </div>
    {/if}
  {/if}
</div>

<ConfirmModal
  bind:open={showDeleteConfirm}
  title="Delete workflow"
  items={pendingDeleteWorkflow ? [pendingDeleteWorkflow.name] : []}
  message="This workflow and all its run history will be permanently deleted."
  onConfirm={confirmDelete}
  onCancel={() => {
    showDeleteConfirm = false
    pendingDeleteWorkflow = null
  }}
/>

<style>
  .wf-add-group {
    border-radius: 8px;
    box-shadow: 0 1px 2px 0 rgb(0 0 0 / 0.08);
  }

  .wf-add-primary {
    display: flex;
    cursor: pointer;
    align-items: center;
    gap: 6px;
    border-radius: 8px 0 0 8px;
    background: var(--accent-cta);
    padding: 7px 13px;
    font-size: 0.875rem;
    font-weight: 500;
    color: #fff;
    transition:
      background 0.15s ease,
      transform 0.08s ease;
    user-select: none;
  }
  .wf-add-primary:hover:not(:disabled) {
    background: var(--accent-cta-hover);
  }
  .wf-add-primary:active:not(:disabled) {
    transform: translateY(1px);
    background: var(--accent-cta-active);
  }
  .wf-add-primary:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .wf-add-chevron {
    display: flex;
    align-items: center;
    border-radius: 0 8px 8px 0;
    border-left: 1px solid rgb(255 255 255 / 0.18);
    background: var(--accent-cta);
    padding: 7px 8px;
    color: #fff;
    cursor: pointer;
    transition:
      background 0.15s ease,
      transform 0.08s ease;
  }
  .wf-add-chevron:hover:not(:disabled) {
    background: var(--accent-cta-hover);
  }
  .wf-add-chevron:active:not(:disabled) {
    transform: translateY(1px);
  }
  .wf-add-chevron:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .wf-add-dropdown {
    position: absolute;
    top: calc(100% + 6px);
    right: 0;
    z-index: 50;
    min-width: 280px;
    border-radius: 10px;
    border: 1px solid var(--border);
    background: var(--bg-surface);
    padding: 4px 0;
    box-shadow:
      0 0 0 1px rgb(0 0 0 / 0.04),
      0 4px 8px -2px rgb(0 0 0 / 0.12),
      0 16px 32px -8px rgb(0 0 0 / 0.18);
    animation: wf-dropdown-in 0.14s cubic-bezier(0.25, 1, 0.5, 1) both;
    transform-origin: top right;
  }

  :global(.dark) .wf-add-dropdown {
    box-shadow:
      0 0 0 1px rgb(0 0 0 / 0.3),
      0 4px 8px -2px rgb(0 0 0 / 0.4),
      0 16px 32px -8px rgb(0 0 0 / 0.5);
  }

  @keyframes wf-dropdown-in {
    from {
      opacity: 0;
      transform: scale(0.95) translateY(-4px);
    }
    to {
      opacity: 1;
      transform: scale(1) translateY(0);
    }
  }

  .wf-add-item {
    display: flex;
    width: 100%;
    align-items: flex-start;
    gap: 10px;
    padding: 8px 12px;
    text-align: left;
    cursor: pointer;
    transition: background 0.08s ease;
  }
  .wf-add-item:hover {
    background: var(--bg-hover);
  }
  :global(.dark) .wf-add-item:hover {
    background: var(--bg-elevated);
  }

  :global(.wf-add-item-icon) {
    width: 14px;
    height: 14px;
    flex-shrink: 0;
    margin-top: 3px;
    color: var(--text-muted);
    transition: color 0.08s ease;
  }
  .wf-add-item:hover :global(.wf-add-item-icon) {
    color: var(--accent-cta);
  }
</style>
