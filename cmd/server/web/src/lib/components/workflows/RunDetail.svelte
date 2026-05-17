<script lang="ts">
  import { openThumbnailEvents } from '$lib/api/client'
  import type {
    WorkflowEvent,
    WorkflowGraph,
    WorkflowRun,
    WorkflowRunStep,
    WorkflowNodeSchema,
  } from '$lib/api/workflows'
  import WorkflowCanvas from './WorkflowCanvas.svelte'

  interface Props {
    run: WorkflowRun | null
    graph: WorkflowGraph | null
    schemas: WorkflowNodeSchema[]
    onRefresh?: () => Promise<void> | void
  }

  let { run, graph, schemas, onRefresh = async () => {} }: Props = $props()

  const stepStatuses = $derived.by(() => {
    const out: Record<string, string> = {}
    for (const step of run?.steps ?? []) {
      out[step.node_id] = step.status
    }
    return out
  })

  const stepCtx = $derived.by(() => {
    const out: Record<string, { input: Record<string, unknown>; output: Record<string, unknown> }> = {}
    for (const step of run?.steps ?? []) {
      out[step.node_id] = { input: step.input_ctx, output: step.output_ctx }
    }
    return out
  })

  function statusClass(status: string) {
    switch (status) {
      case 'completed':
        return 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
      case 'failed':
        return 'bg-rose-500/10 text-rose-700 dark:text-rose-300'
      case 'running':
        return 'bg-sky-500/10 text-sky-700 dark:text-sky-300'
      default:
        return 'bg-slate-500/10 text-slate-700 dark:text-slate-300'
    }
  }

  function duration(step: WorkflowRunStep) {
    if (!step.started_at || !step.completed_at) return 'Pending'
    const ms =
      new Date(step.completed_at).getTime() -
      new Date(step.started_at).getTime()
    if (ms < 1000) return `${ms} ms`
    return `${(ms / 1000).toFixed(1)} s`
  }

  function pretty(value: Record<string, unknown>) {
    return JSON.stringify(value, null, 2)
  }

  $effect(() => {
    if (!run || typeof window === 'undefined') return
    const source = openThumbnailEvents()
    const listener = (event: MessageEvent) => {
      try {
        const payload = JSON.parse(event.data) as WorkflowEvent
        if (
          (payload.type === 'workflow_run_step_updated' ||
            payload.type === 'workflow_run_failed') &&
          payload.run_id === run.id
        ) {
          void onRefresh()
        }
      } catch {
        // Ignore unrelated SSE payloads.
      }
    }
    source.addEventListener('message', listener)
    return () => {
      source.removeEventListener('message', listener)
      source.close()
    }
  })
</script>

<div class="space-y-4">
  <div
    class="rounded-[24px] border border-[var(--border-subtle)] bg-[var(--bg-surface)]"
  >
    <div class="border-b border-[var(--border-subtle)] px-4 py-4">
      <h3 class="text-sm font-semibold text-[var(--text-primary)]">
        Run Detail
      </h3>
      <p class="mt-1 text-sm text-[var(--text-secondary)]">
        Follow execution status node-by-node on the workflow graph.
      </p>
    </div>

    {#if run === null || graph === null}
      <div class="px-4 py-6 text-sm text-[var(--text-muted)]">
        Select a run to inspect step-by-step execution.
      </div>
    {:else}
      <div class="space-y-4 px-4 py-4">
        <div class="grid gap-3 lg:grid-cols-3">
          <div>
            <p
              class="text-[11px] font-semibold tracking-[0.16em] text-[var(--text-muted)] uppercase"
            >
              Status
            </p>
            <span
              class="mt-2 inline-flex rounded-full px-2.5 py-1 text-[10px] font-semibold uppercase {statusClass(
                run.status
              )}"
            >
              {run.status}
            </span>
          </div>
          <div>
            <p
              class="text-[11px] font-semibold tracking-[0.16em] text-[var(--text-muted)] uppercase"
            >
              Started
            </p>
            <p class="mt-2 text-sm text-[var(--text-primary)]">
              {run.started_at
                ? new Date(run.started_at).toLocaleString()
                : 'Pending'}
            </p>
          </div>
          <div>
            <p
              class="text-[11px] font-semibold tracking-[0.16em] text-[var(--text-muted)] uppercase"
            >
              Completed
            </p>
            <p class="mt-2 text-sm text-[var(--text-primary)]">
              {run.completed_at
                ? new Date(run.completed_at).toLocaleString()
                : 'Still running'}
            </p>
          </div>
        </div>

        {#if run.error}
          <div
            class="rounded-2xl border border-rose-500/20 bg-rose-500/8 px-4 py-3 text-sm text-rose-700 dark:text-rose-300"
          >
            {run.error}
          </div>
        {/if}

        <WorkflowCanvas {graph} {schemas} readonly={true} {stepStatuses} {stepCtx} />
      </div>
    {/if}
  </div>

  {#if run}
    <div
      class="rounded-[24px] border border-[var(--border-subtle)] bg-[var(--bg-surface)]"
    >
      <div class="border-b border-[var(--border-subtle)] px-4 py-4">
        <h4 class="text-sm font-semibold text-[var(--text-primary)]">
          Step Timeline
        </h4>
      </div>

      <div class="px-4 py-4">
        {#if run.steps.length === 0}
          <p class="text-sm text-[var(--text-muted)]">No steps recorded yet.</p>
        {:else}
          <div class="relative space-y-0">
            <div
              class="absolute top-3 bottom-3 left-[19px] w-px bg-[var(--border-subtle)]"
            ></div>
            {#each run.steps as step (step.node_id + step.attempt)}
              <details class="group relative pl-10">
                <div
                  class="absolute top-[18px] left-[13px] h-3 w-3 rounded-full border-2 {step.status ===
                  'completed'
                    ? 'border-emerald-500 bg-emerald-500/20'
                    : step.status === 'failed'
                      ? 'border-rose-500 bg-rose-500/20'
                      : step.status === 'running'
                        ? 'border-sky-500 bg-sky-500/20'
                        : 'border-[var(--border-default)] bg-[var(--bg-elevated)]'}"
                ></div>
                <summary
                  class="cursor-pointer list-none rounded-xl py-2 pr-2 transition-colors hover:bg-[var(--bg-hover)]"
                >
                  <div class="flex items-start justify-between gap-3">
                    <div class="min-w-0">
                      <p
                        class="text-sm font-semibold text-[var(--text-primary)]"
                      >
                        {step.node_type}
                      </p>
                      <p
                        class="mt-0.5 font-mono text-[10px] text-[var(--text-muted)]"
                      >
                        {step.node_id} · attempt {step.attempt} · {duration(
                          step
                        )}
                      </p>
                    </div>
                    <span
                      class="shrink-0 rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase {statusClass(
                        step.status
                      )}"
                    >
                      {step.status}
                    </span>
                  </div>
                </summary>

                <div class="pt-2 pb-3">
                  {#if step.error}
                    <p
                      class="mb-3 rounded-lg bg-rose-500/8 px-3 py-2 text-sm text-rose-700 dark:text-rose-300"
                    >
                      {step.error}
                    </p>
                  {/if}

                  <div class="grid gap-3 xl:grid-cols-2">
                    <div>
                      <p
                        class="mb-1.5 text-[11px] font-semibold tracking-[0.14em] text-[var(--text-muted)] uppercase"
                      >
                        Input
                      </p>
                      <pre
                        class="overflow-x-auto rounded-lg bg-[var(--bg-elevated)] px-3 py-2.5 font-mono text-[11px] text-[var(--text-primary)]">{pretty(
                          step.input_ctx
                        )}</pre>
                    </div>
                    <div>
                      <p
                        class="mb-1.5 text-[11px] font-semibold tracking-[0.14em] text-[var(--text-muted)] uppercase"
                      >
                        Output
                      </p>
                      <pre
                        class="overflow-x-auto rounded-lg bg-[var(--bg-elevated)] px-3 py-2.5 font-mono text-[11px] text-[var(--text-primary)]">{pretty(
                          step.output_ctx
                        )}</pre>
                    </div>
                  </div>
                </div>
              </details>
            {/each}
          </div>
        {/if}
      </div>
    </div>
  {/if}
</div>
