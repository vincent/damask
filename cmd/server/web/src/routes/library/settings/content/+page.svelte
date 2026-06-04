<script lang="ts">
  import { workspaceApi } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import Modal from '$lib/components/ui/Modal.svelte'
  import PageHeader from '$lib/components/ui/PageHeader.svelte'
  import PageContainer from '$lib/components/ui/PageContainer.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import { m } from '$lib/paraglide/messages'

  const isOwner = $derived(authStore.role === 'owner')
  let running = $state(false)
  let showConfirm = $state(false)

  async function triggerBackfill() {
    showConfirm = false
    running = true
    try {
      await workspaceApi.triggerJob('visual-similarity-backfill')
      toastStore.show('Visual similarity backfill queued')
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : 'Failed to queue job',
        'error'
      )
    } finally {
      running = false
    }
  }
</script>

<svelte:head>
  <title>{m.settings_content_title()} — Damask</title>
</svelte:head>

<PageContainer>
  <PageHeader
    title={m.settings_content_title()}
    description={m.settings_content_description()}
  />
  <div class="mx-auto w-full max-w-3xl space-y-8 px-8 py-10">
    {#if !isOwner}
      <p
        class="text-md rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400"
      >
        {m.version_history_settins_only_owners()}
      </p>
    {/if}

    <div
      class="space-y-6 rounded-xl border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-6 shadow-sm"
    >
      <div class="flex items-start justify-between gap-4">
        <div class="flex-1">
          <p class="text-md font-medium text-[var(--text-primary)]">
            {m.visual_similarity_backfill()}
          </p>
          <p class="mt-0.5 text-sm text-[var(--text-muted)]">
            {m.visual_similarity_backfill_description()}
          </p>
        </div>
        <Button
          variant="secondary"
          disabled={running || !isOwner}
          loading={running}
          onclick={() => (showConfirm = true)}
        >
          {m.visual_similarity_backfill_run()}
        </Button>
      </div>
    </div>
  </div>
</PageContainer>

<Modal bind:open={showConfirm} onclose={() => (showConfirm = false)}>
  <div class="p-6">
    <h2 class="mb-3 text-lg font-semibold text-[var(--text-primary)]">
      {m.visual_similarity_backfill()}?
    </h2>
    <p class="text-md text-[var(--text-secondary)]">
      {m.visual_similarity_backfill_description()}
    </p>
    <div class="mt-6 flex justify-end gap-3">
      <Button variant="secondary" onclick={() => (showConfirm = false)}>
        {m.cancel()}
      </Button>
      <Button variant="primary" onclick={triggerBackfill}>
        {m.visual_similarity_backfill_run()}
      </Button>
    </div>
  </div>
</Modal>
