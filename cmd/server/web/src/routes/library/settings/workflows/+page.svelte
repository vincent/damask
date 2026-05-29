<script lang="ts">
  import { page } from '$app/state'
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
    Activity,
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
  } from '$lib/api/workflows'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { workflowsStore } from '$lib/stores/workflows.svelte'
  import { m } from '$lib/paraglide/messages'

  type View = 'list' | 'editor'

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
      const requestedWorkflowID = page.url.searchParams.get('workflow')
      if (requestedWorkflowID) {
        const workflow = workflowsStore.workflows.find(
          (row) => row.id === requestedWorkflowID
        )
        if (workflow) openEditor(workflow)
      }
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : m.load_page_failed(),
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
    view = 'editor'
  }

  function goToList() {
    view = 'list'
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
        e instanceof Error ? e.message : m.workflow_create_failed(),
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
      toastStore.show(m.workflow_saved(workflow))
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : m.workflow_save_failed(),
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
        e instanceof Error ? e.message : m.workflow_update_failed(),
        'error'
      )
    } finally {
      toggling = false
    }
  }

  async function runWorkflow(workflow: Workflow) {
    try {
      await workflowsApi.triggerManual(workflow.id)
      toastStore.show(m.workflow_triggered(workflow))
      if (panelWorkflow?.id === workflow.id) {
        panelWorkflow = { ...panelWorkflow }
      }
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : m.workflow_trigger_failed(),
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
        e instanceof Error ? e.message : m.workflow_delete_failed(),
        'error'
      )
    } finally {
      deleting = false
    }
  }
</script>

<svelte:head>
  <title>{m.workflows()} — Damask</title>
</svelte:head>

<div class="flex flex-1 flex-col overflow-hidden">
  {#if view === 'list'}
    <!-- ── LIST VIEW ─────────────────────────────────────────── -->
    <PageHeader title={m.workflows()} description={m.workflow_subtitle()}>
      <a
        href="/library/settings/workflows/runs"
        class="flex items-center gap-1.5 rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-1.5 text-sm text-[var(--text-secondary)] transition-colors hover:text-[var(--text-primary)]"
      >
        <Activity class="h-3.5 w-3.5" />
        {m.workflows_runs_all()}
      </a>
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
          {m.add_workflow()}
        </button>
        <button
          type="button"
          disabled={creating}
          class="wf-add-chevron"
          onclick={() => (addMenuOpen = !addMenuOpen)}
          aria-label={m.workflow_choose_template()}
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
            title={m.workflows_empty()}
            description={m.workflow_add_help()}
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
        />
      {/if}
    </div>
  {:else}
    <!-- ── EDITOR VIEW ───────────────────────────────────────── -->
    <PageHeader
      title={selectedWorkflow?.name ?? m.editor()}
      description={m.workflow_edit_help()}
    >
      {#snippet meta()}
        {#if dirty !== undefined}
          <div
            class="mt-0.5 inline-flex items-center gap-1.5 text-[11px] {dirty
              ? 'text-amber-500'
              : 'text-transparent'}"
          >
            <span class="h-1.5 w-1.5 rounded-full {dirty ? 'bg-amber-400' : ''}"
            ></span>
            {dirty ? m.unsaved_changes() : '·'}
          </div>
        {/if}
      {/snippet}
      <div class="flex items-center gap-2">
        <Button variant="ghost" onclick={goToList}>
          {#snippet icon()}<ArrowLeft class="h-4 w-4" />{/snippet}
          {m.back()}
        </Button>

        {#if selectedWorkflow}
          <Button
            variant="ghost"
            disabled={toggling}
            onclick={() =>
              selectedWorkflow && void toggleWorkflow(selectedWorkflow)}
          >
            {#if selectedWorkflow.enabled}
              <Pause class="h-4 w-4" />
              {m.pause()}
            {:else}
              <Play class="h-4 w-4" />
              {m.enable()}
            {/if}
          </Button>
          <Button
            variant="ghost"
            disabled={deleting}
            onclick={() => selectedWorkflow && handleDelete(selectedWorkflow)}
            class="text-red-500 hover:text-red-600"
          >
            <Trash2 class="h-4 w-4" />
            {m.delete()}
          </Button>
          <Button
            disabled={!dirty || saving}
            onclick={() => void saveWorkflow()}
          >
            <Save class="h-4 w-4" />
            {saving ? m.saving() : m.save()}
          </Button>
        {/if}
      </div>
    </PageHeader>

    {#if selectedWorkflow}
      <div class="flex min-h-0 flex-1 flex-col gap-6">
        <!-- Editor: name/desc/email + canvas -->
        <div
          class="border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-5 pt-0"
        >
          <div class="grid gap-4 lg:grid-cols-3">
            <Input label={m.name()} bind:value={draftName} />
            <Input label={m.description()} bind:value={draftDescription} />
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
  title={m.workflow_delete()}
  items={pendingDeleteWorkflow ? [pendingDeleteWorkflow.name] : []}
  message={m.workflow_delete_help()}
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
