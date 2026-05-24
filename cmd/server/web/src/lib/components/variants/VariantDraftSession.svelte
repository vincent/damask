<script lang="ts">
  import { onDestroy } from 'svelte'
  import {
    commitDraft,
    discardDraft,
    createDraftSubscription,
    checkDraftEvent,
    type DraftSubscription,
  } from '$lib/api/drafts'
  import { sseEvents } from '$lib/stores/assets.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import { m } from '$lib/paraglide/messages'
  import VariantDraftCard from './VariantDraftCard.svelte'

  interface Props {
    assetId: string
    onDone: () => void
    onAddMore: () => void
    gridMode?: boolean
  }

  let { assetId, onDone, onAddMore, gridMode = false }: Props = $props()

  interface DraftEntry {
    nonce: string
    phase: 'generating' | 'ready' | 'error'
    previewUrl: string
    expiresAt: string
    errorMsg: string
    sub: DraftSubscription
    timeoutId: ReturnType<typeof setTimeout>
  }

  let drafts = $state<DraftEntry[]>([])
  let committingNonces = $state(new Set<string>())
  let keepAllProgress = $state<{ current: number; total: number } | null>(null)

  $effect(() => {
    const event = sseEvents.last
    for (const d of drafts) {
      checkDraftEvent(d.sub, event)
    }
  })

  function handleDraftEvent(
    nonce: string,
    e: {
      type: string
      preview_url?: string
      expires_at?: string
      error?: string
    }
  ) {
    drafts = drafts.map((d) => {
      if (d.nonce !== nonce) return d
      clearTimeout(d.timeoutId)
      if (e.type === 'variant_draft.ready') {
        return {
          ...d,
          phase: 'ready',
          previewUrl: e.preview_url ?? '',
          expiresAt: e.expires_at ?? '',
        }
      }
      return {
        ...d,
        phase: 'error',
        errorMsg: m.variants_draft_error_generating({ error: e.error ?? '' }),
      }
    })
  }

  function handleTimeout(nonce: string) {
    drafts = drafts.map((d) => {
      if (d.nonce !== nonce) return d
      d.sub.done = true
      return { ...d, phase: 'error', errorMsg: m.variants_draft_timed_out() }
    })
  }

  export function addDraft(nonce: string) {
    const sub = createDraftSubscription(nonce, (e) =>
      handleDraftEvent(nonce, e)
    )
    const timeoutId = setTimeout(() => handleTimeout(nonce), 120_000)
    drafts = [
      ...drafts,
      {
        nonce,
        phase: 'generating',
        previewUrl: '',
        expiresAt: '',
        errorMsg: '',
        sub,
        timeoutId,
      },
    ]
  }

  async function handleKeep(nonce: string) {
    committingNonces = new Set([...committingNonces, nonce])
    try {
      await commitDraft(assetId, nonce)
      drafts = drafts.filter((d) => d.nonce !== nonce)
      toastStore.show(m.variants_draft_committed(), 'success')
      if (drafts.length === 0) onAddMore()
    } catch {
      toastStore.show(m.variants_draft_commit_error(), 'error')
    } finally {
      committingNonces = new Set(
        [...committingNonces].filter((n) => n !== nonce)
      )
    }
  }

  async function handleKeepAll() {
    const ready = drafts.filter((d) => d.phase === 'ready')
    keepAllProgress = { current: 0, total: ready.length }
    for (const [i, draft] of ready.entries()) {
      committingNonces = new Set([...committingNonces, draft.nonce])
      keepAllProgress = { current: i + 1, total: ready.length }
      try {
        await commitDraft(assetId, draft.nonce)
      } catch (e) {
        console.error('Failed to commit draft', draft.nonce, e)
        toastStore.show(m.variants_draft_commit_error(), 'error')
        keepAllProgress = null
        committingNonces = new Set(
          [...committingNonces].filter((n) => n !== draft.nonce)
        )
        return
      }
      committingNonces = new Set(
        [...committingNonces].filter((n) => n !== draft.nonce)
      )
    }
    keepAllProgress = null
    toastStore.show(
      m.variants_draft_all_committed({ n: String(ready.length) }),
      'success'
    )
    onDone()
  }

  function handleDiscard(nonce: string) {
    discardDraft(assetId, nonce).catch(() => {})
    drafts = drafts.filter((d) => d.nonce !== nonce)
    if (drafts.length === 0) onAddMore()
  }

  const readyCount = $derived(drafts.filter((d) => d.phase === 'ready').length)
  const isBusy = $derived(committingNonces.size > 0)

  onDestroy(() => {
    for (const d of drafts) {
      d.sub.done = true
      clearTimeout(d.timeoutId)
    }
  })
</script>

<div class="draft-session" class:grid-mode={gridMode}>
  <p class="session-title">{m.variants_draft_session_title()}</p>
  <p class="session-subtitle text-sm">{m.variants_draft_add_more()}</p>

  <div class="draft-list" class:single={drafts.length === 1}>
    {#each drafts as draft (draft.nonce)}
      <VariantDraftCard
        {assetId}
        nonce={draft.nonce}
        previewUrl={draft.previewUrl}
        expiresAt={draft.expiresAt}
        errorMsg={draft.errorMsg}
        phase={draft.phase}
        isCommitting={committingNonces.has(draft.nonce)}
        onKeep={() => handleKeep(draft.nonce)}
        onDiscard={() => handleDiscard(draft.nonce)}
      />
    {/each}
  </div>

  <div class="session-footer">
    {#if readyCount >= 2}
      <button
        type="button"
        class="btn-keep-all"
        disabled={isBusy}
        onclick={handleKeepAll}
      >
        {#if keepAllProgress}
          {m.variants_draft_keeping_n({
            current: String(keepAllProgress.current),
            total: String(keepAllProgress.total),
          })}
        {:else}
          {m.variants_draft_keep_all({ n: String(readyCount) })}
        {/if}
      </button>
    {/if}
  </div>
</div>

<style>
  .draft-session {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .session-title {
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .session-subtitle {
    font-size: 0.7125rem;
    font-weight: 600;
    color: var(--text-secondary);
    letter-spacing: 0.04em;
  }

  .draft-list {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .grid-mode .draft-list {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
    gap: 16px;
  }

  .grid-mode .draft-list.single {
    grid-template-columns: 1fr;
  }

  .session-footer {
    display: flex;
    align-items: center;
    justify-content: end;
    gap: 8px;
    padding-top: 4px;
  }

  .btn-add-more {
    font-size: 0.8125rem;
    color: var(--accent-cta);
    background: none;
    border: none;
    padding: 0;
    cursor: pointer;
    transition: opacity 120ms ease;
  }

  .btn-add-more:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }

  .btn-keep-all {
    padding: 5px 14px;
    border-radius: 6px;
    border: none;
    background: var(--accent-cta);
    color: #fff;
    font-size: 0.8125rem;
    font-weight: 500;
    cursor: pointer;
    transition: opacity 120ms ease;
  }

  .btn-keep-all:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }
</style>
