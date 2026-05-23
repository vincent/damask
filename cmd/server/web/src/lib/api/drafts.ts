import { apiFetch } from './client'
import type { Variant } from './models'

export interface DraftGenerateResponse {
  draft_key: string
}

export interface DraftReadyEvent {
  type: 'variant_draft.ready'
  nonce: string
  preview_url: string
  expires_at: string
}

export interface DraftErrorEvent {
  type: 'variant_draft.error'
  nonce: string
  error: string
}

export type DraftEvent = DraftReadyEvent | DraftErrorEvent

/** POST /assets/:id/variants/draft — enqueue draft job, returns draft_key immediately (202). */
export async function generateDraft(
  assetId: string,
  type: string,
  params: Record<string, unknown>
): Promise<DraftGenerateResponse> {
  return apiFetch<DraftGenerateResponse>(
    `/api/v1/assets/${assetId}/variants/draft`,
    {
      method: 'POST',
      body: JSON.stringify({ type, params }),
    }
  )
}

export interface DraftSubscription {
  nonce: string
  handler: (e: DraftEvent) => void
  done: boolean
}

/**
 * Create a draft event subscription record.
 * The caller must run a $effect that calls checkDraftEvent(sub, sseEvents.last).
 */
export function createDraftSubscription(
  nonce: string,
  handler: (e: DraftEvent) => void
): DraftSubscription {
  return { nonce, handler, done: false }
}

/**
 * Check a single SSE event against a subscription.
 * Call this inside a component $effect whenever sseEvents.last changes.
 */
export function checkDraftEvent(
  sub: DraftSubscription,
  event: { type: string; nonce?: string } | null
): void {
  if (!event || sub.done) return
  if (event.nonce !== sub.nonce) return
  if (
    event.type === 'variant_draft.ready' ||
    event.type === 'variant_draft.error'
  ) {
    sub.done = true
    sub.handler(event as unknown as DraftEvent)
  }
}

/** POST /assets/:id/variants/draft/:nonce/commit — move scratch → permanent, insert variant row. */
export async function commitDraft(
  assetId: string,
  nonce: string,
  name?: string
): Promise<Variant> {
  return apiFetch<Variant>(
    `/api/v1/assets/${assetId}/variants/draft/${nonce}/commit`,
    {
      method: 'POST',
      body: JSON.stringify({ name: name ?? '' }),
    }
  )
}

/** DELETE /assets/:id/variants/draft/:nonce — idempotent discard, returns 204. */
export async function discardDraft(
  assetId: string,
  nonce: string
): Promise<void> {
  await apiFetch<void>(`/api/v1/assets/${assetId}/variants/draft/${nonce}`, {
    method: 'DELETE',
  })
}
