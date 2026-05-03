<script lang="ts">
  import { workspaceApi } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import Modal from '$lib/components/ui/Modal.svelte'
  import PageHeader from '$lib/components/ui/PageHeader.svelte'
  import PageContainer from '$lib/components/ui/PageContainer.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import { m } from '$lib/paraglide/messages'

  const ws = $derived(authStore.workspace)
  const isOwner = $derived(authStore.role === 'owner')

  let exifKeep = $state(false)
  let exifKeepGPS = $state(false)
  let saving = $state(false)
  let showBackfillConfirm = $state(false)
  let backfilling = $state(false)

  $effect(() => {
    if (ws) {
      exifKeep = ws.exif_keep ?? false
      exifKeepGPS = ws.exif_keep_gps ?? false
    }
  })

  const isDirty = $derived(
    ws
      ? exifKeep !== (ws.exif_keep ?? false) ||
          exifKeepGPS !== (ws.exif_keep_gps ?? false)
      : false
  )

  async function save() {
    saving = true
    try {
      const updated = await workspaceApi.updateSettings({
        version_retention_count: ws?.version_retention_count ?? 0,
        exif_keep: !!exifKeep,
        exif_keep_gps: !!exifKeepGPS,
      })
      authStore.patchWorkspace({
        exif_keep: !!updated.exif_keep,
        exif_keep_gps: !!updated.exif_keep_gps,
      })
      toastStore.show(m.privacy_settings_saved())
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : m.privacy_settings_failed(),
        'error'
      )
    } finally {
      saving = false
    }
  }

  async function triggerBackfill() {
    showBackfillConfirm = false
    backfilling = true
    try {
      const result = await workspaceApi.triggerJob('extract_exif')
      if (result.enqueued === 0) {
        toastStore.show('All assets already have EXIF data extracted')
      } else {
        toastStore.show(
          `Queued EXIF extraction for ${result.enqueued} asset${result.enqueued === 1 ? '' : 's'}`
        )
      }
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : 'Failed to queue backfill',
        'error'
      )
    } finally {
      backfilling = false
    }
  }
</script>

<svelte:head>
  <title>{m.tab_exif_privacy()} — Damask</title>
</svelte:head>

<PageContainer>
  <PageHeader
    title={m.tab_exif_privacy()}
    description={m.tab_exif_privacy_description()}
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
      <!-- EXIF extraction toggle -->
      <div class="flex items-start justify-between gap-4">
        <div class="flex-1">
          <p class="text-md font-medium text-[var(--text-primary)]">
            {m.extract_exif()}
          </p>
          <p class="mt-0.5 text-sm text-[var(--text-muted)]">
            {m.extract_exif_description()}
          </p>
        </div>
        <button
          type="button"
          role="switch"
          aria-checked={exifKeep}
          aria-label={m.extract_exif()}
          disabled={!isOwner}
          onclick={() => {
            if (isOwner) exifKeep = !exifKeep
          }}
          class="relative mt-0.5 inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors focus:outline-none disabled:cursor-not-allowed disabled:opacity-50
            {exifKeep ? 'bg-indigo-600' : 'bg-gray-200 dark:bg-gray-700'}"
        >
          <span
            class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow ring-0 transition-transform
            {exifKeep ? 'translate-x-5' : 'translate-x-0'}"
          ></span>
        </button>
      </div>

      <!-- GPS toggle — only shown when EXIF enabled -->
      {#if exifKeep}
        <div
          class="flex items-start justify-between gap-4 border-t border-[var(--border-subtle)] pt-6"
        >
          <div class="flex-1">
            <p class="text-md font-medium text-[var(--text-primary)]">
              {m.keep_gps()}
            </p>
            <p class="mt-0.5 text-sm text-[var(--text-muted)]">
              {m.keep_gps_description()}
            </p>
          </div>
          <button
            type="button"
            role="switch"
            aria-checked={exifKeepGPS}
            aria-label={m.keep_gps()}
            disabled={!isOwner}
            onclick={() => {
              if (isOwner) exifKeepGPS = !exifKeepGPS
            }}
            class="relative mt-0.5 inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors focus:outline-none disabled:cursor-not-allowed disabled:opacity-50
              {exifKeepGPS ? 'bg-indigo-600' : 'bg-gray-200 dark:bg-gray-700'}"
          >
            <span
              class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow ring-0 transition-transform
              {exifKeepGPS ? 'translate-x-5' : 'translate-x-0'}"
            ></span>
          </button>
        </div>
      {/if}
    </div>

    {#if isOwner}
      <div class="flex items-center justify-between">
        <Button
          variant="secondary"
          disabled={backfilling || !exifKeep}
          loading={backfilling}
          title={!exifKeep ? m.extract_exif_first() : undefined}
          onclick={() => (showBackfillConfirm = true)}
        >
          {m.extract_exif_again()}
        </Button>

        <Button
          variant="primary"
          disabled={!isDirty}
          loading={saving}
          onclick={save}
        >
          {m.save()}
        </Button>
      </div>
    {/if}
  </div>
</PageContainer>

{#if showBackfillConfirm}
  <Modal onclose={() => (showBackfillConfirm = false)}>
    <h2 class="mb-3 text-lg font-semibold text-[var(--text-primary)]">
      {m.extract_exif_again()}?
    </h2>
    <p class="text-md text-[var(--text-secondary)]">
      {m.extract_exif_again_description()}
    </p>
    <div class="mt-6 flex justify-end gap-3">
      <Button variant="secondary" onclick={() => (showBackfillConfirm = false)}>
        {m.cancel()}
      </Button>
      <Button variant="primary" onclick={triggerBackfill}>
        {m.reextract()}
      </Button>
    </div>
  </Modal>
{/if}
