<script lang="ts">
  import { onMount } from 'svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import Input from '$lib/components/ui/Input.svelte'
  import EmptyState from '$lib/components/ui/EmptyState.svelte'
  import PageHeader from '$lib/components/ui/PageHeader.svelte'
  import WorkflowCanvas from '$lib/components/workflows/WorkflowCanvas.svelte'
  import {
    workflowsApi,
    type Workflow,
    type WorkflowGraph,
    type WorkflowTemplate,
    type WorkflowNodeSchema,
  } from '$lib/api/workflows'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { workflowsStore } from '$lib/stores/workflows.svelte'
  import { ChevronDown, Pause, Play, Plus, Save, Sparkles, Trash2 } from '@lucide/svelte'

  let creating = $state(false)
  let saving = $state(false)
  let toggling = $state(false)
  let deleting = $state(false)
  let templates = $state<WorkflowTemplate[]>([])
  let nodeSchemas = $state<WorkflowNodeSchema[]>([])
  let addMenuOpen = $state(false)

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

  const selectedWorkflow = $derived(
    workflowsStore.workflows.find((w) => w.id === workflowsStore.selectedId) ??
      null
  )
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
    try {
      const [templateRows, schemaRows] = await Promise.all([
        workflowsApi.getTemplates(),
        workflowsApi.getNodeSchemas(),
      ])
      templates = templateRows
      nodeSchemas = schemaRows
      await workflowsStore.load()

      const rows = workflowsStore.workflows
      const nextId =
        workflowsStore.selectedId &&
        rows.some((r) => r.id === workflowsStore.selectedId)
          ? workflowsStore.selectedId
          : (rows[0]?.id ?? null)
      if (nextId) selectWorkflow(nextId)
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : 'Failed to load workflows.',
        'error'
      )
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

  function selectWorkflow(workflowId: string) {
    const workflow = workflowsStore.workflows.find((w) => w.id === workflowId)
    if (!workflow) return
    workflowsStore.selectedId = workflow.id
    setEditorState(workflow)
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
      selectWorkflow(workflow.id)
      toastStore.show(`Workflow "${workflow.name}" created.`)
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

  async function toggleWorkflow() {
    if (!selectedWorkflow) return
    toggling = true
    try {
      await workflowsApi.setEnabled(selectedWorkflow.id, !selectedWorkflow.enabled)
      workflowsStore.patch(selectedWorkflow.id, { enabled: !selectedWorkflow.enabled })
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : 'Failed to update workflow.',
        'error'
      )
    } finally {
      toggling = false
    }
  }

  async function deleteWorkflow() {
    if (!selectedWorkflow) return
    if (!confirm(`Delete workflow "${selectedWorkflow.name}"? This cannot be undone.`)) return
    deleting = true
    try {
      await workflowsApi.delete(selectedWorkflow.id)
      workflowsStore.remove(selectedWorkflow.id)
      const next = workflowsStore.workflows[0] ?? null
      if (next) selectWorkflow(next.id)
      else workflowsStore.selectedId = null
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
  <PageHeader
    title="Workflows"
    description="Automate DAM actions with trigger, filter, action, and control-flow graphs."
  >
    <div class="relative">
      <button
        type="button"
        disabled={creating}
        class="flex items-center gap-1.5 rounded-lg border border-[var(--border-default)] bg-[var(--bg-surface)] px-3 py-1.5 text-sm font-medium text-[var(--text-primary)] shadow-sm transition-colors hover:bg-[var(--bg-hover)] disabled:opacity-50"
        onclick={() => (addMenuOpen = !addMenuOpen)}
        aria-haspopup="listbox"
        aria-expanded={addMenuOpen}
      >
        <Plus class="h-3.5 w-3.5" />
        Add workflow
        <ChevronDown
          class="h-3.5 w-3.5 text-[var(--text-muted)] transition-transform duration-150 {addMenuOpen
            ? 'rotate-180'
            : ''}"
        />
      </button>

      {#if addMenuOpen}
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div
          class="fixed inset-0 z-40"
          onclick={() => (addMenuOpen = false)}
          onkeydown={(e) => e.key === 'Escape' && (addMenuOpen = false)}
        ></div>
        <div
          class="absolute top-full right-0 z-50 mt-1.5 w-72 rounded-xl border border-[var(--border-subtle)] bg-[var(--bg-surface)] py-1.5 shadow-xl"
        >
          {#each templates as template (template.id)}
            <button
              type="button"
              class="flex w-full items-start gap-3 px-3 py-2.5 text-left transition-colors hover:bg-[var(--bg-hover)]"
              onclick={() => void createFromTemplate(template)}
            >
              <Sparkles
                class="mt-0.5 h-3.5 w-3.5 shrink-0 text-[var(--accent)]"
              />
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

  <div class="flex min-h-0 flex-1 flex-col gap-6 overflow-y-auto p-6">
    {#if !selectedWorkflow}
      <div
        class="rounded-[28px] border border-dashed border-[var(--border-subtle)] bg-[var(--bg-surface)] px-6"
      >
        <EmptyState
          title="No workflow selected"
          description={'Pick a workflow from the sidebar or use "Add workflow" to create one.'}
        />
      </div>
    {:else}
      <div
        class="rounded-[28px] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-5"
      >
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="min-w-0">
            <h2
              class="truncate text-lg font-semibold text-[var(--text-primary)]"
              data-workflow-id={selectedWorkflow.id}
              data-trigger-type={selectedWorkflow.trigger_type}
            >
              {selectedWorkflow.name}
            </h2>
            {#if dirty}
              <span
                class="mt-0.5 inline-flex items-center gap-1.5 text-[11px] text-amber-500"
              >
                <span class="h-1.5 w-1.5 rounded-full bg-amber-400"></span>
                Unsaved changes
              </span>
            {/if}
          </div>

          <div class="flex items-center gap-2">
            <Button
              variant="ghost"
              disabled={toggling}
              onclick={() => void toggleWorkflow()}
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
              onclick={() => void deleteWorkflow()}
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
          </div>
        </div>

        <div class="mt-5 grid gap-4 lg:grid-cols-3">
          <Input label="Name" bind:value={draftName} />
          <Input label="Description" bind:value={draftDescription} />
          <Input
            label="Failure Email"
            bind:value={draftNotifyEmail}
            placeholder="alerts@example.com"
          />
        </div>

        <div class="mt-5">
          <WorkflowCanvas bind:graph={editableGraph} schemas={nodeSchemas} />
        </div>
      </div>
    {/if}
  </div>
</div>
