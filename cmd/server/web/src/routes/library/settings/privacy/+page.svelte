<script lang="ts">
  import { workspaceApi } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import Modal from '$lib/components/ui/Modal.svelte'
  import PageHeader from '$lib/components/ui/PageHeader.svelte'
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
    ws ? exifKeep !== (ws.exif_keep ?? false) || exifKeepGPS !== (ws.exif_keep_gps ?? false) : false
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
      toastStore.show(e instanceof Error ? e.message : m.privacy_settings_failed(), 'error')
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
        toastStore.show(`Queued EXIF extraction for ${result.enqueued} asset${result.enqueued === 1 ? '' : 's'}`)
      }
    } catch (e) {
      toastStore.show(e instanceof Error ? e.message : 'Failed to queue backfill', 'error')
    } finally {
      backfilling = false
    }
  }
</script>

<svelte:head>
  <title>{m.tab_exif_privacy()} — Damask</title>
</svelte:head>

<div class="flex-1 overflow-y-auto">
  <PageHeader title={m.tab_exif_privacy()} description={m.tab_exif_privacy_description()} />
  <div class="mx-auto w-full max-w-3xl px-8 py-10 space-y-8">

    {#if !isOwner}
      <p class="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-md text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400">
        {m.version_history_settins_only_owners()}
      </p>
    {/if}

    <div class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-700 dark:bg-gray-900 space-y-6">

      <!-- EXIF extraction toggle -->
      <div class="flex items-start justify-between gap-4">
        <div class="flex-1">
          <p class="text-md font-medium text-gray-900 dark:text-gray-100">{m.extract_exif()}</p>
          <p class="mt-0.5 text-sm text-gray-500 dark:text-gray-400">{m.extract_exif_description()}</p>
        </div>
        <button
          type="button"
          role="switch"
          aria-checked={exifKeep}
          aria-label={m.extract_exif()}
          disabled={!isOwner}
          onclick={() => { if (isOwner) exifKeep = !exifKeep }}
          class="relative mt-0.5 inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors focus:outline-none disabled:cursor-not-allowed disabled:opacity-50
            {exifKeep ? 'bg-indigo-600' : 'bg-gray-200 dark:bg-gray-700'}"
        >
          <span class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow ring-0 transition-transform
            {exifKeep ? 'translate-x-5' : 'translate-x-0'}"></span>
        </button>
      </div>

      <!-- GPS toggle — only shown when EXIF enabled -->
      {#if exifKeep}
        <div class="flex items-start justify-between gap-4 border-t border-gray-100 pt-6 dark:border-gray-800">
          <div class="flex-1">
            <p class="text-md font-medium text-gray-900 dark:text-gray-100">{m.keep_gps()}</p>
            <p class="mt-0.5 text-sm text-gray-500 dark:text-gray-400">{m.keep_gps_description()}</p>
          </div>
          <button
            type="button"
            role="switch"
            aria-checked={exifKeepGPS}
            aria-label={m.keep_gps()}
            disabled={!isOwner}
            onclick={() => { if (isOwner) exifKeepGPS = !exifKeepGPS }}
            class="relative mt-0.5 inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors focus:outline-none disabled:cursor-not-allowed disabled:opacity-50
              {exifKeepGPS ? 'bg-indigo-600' : 'bg-gray-200 dark:bg-gray-700'}"
          >
            <span class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow ring-0 transition-transform
              {exifKeepGPS ? 'translate-x-5' : 'translate-x-0'}"></span>
          </button>
        </div>
      {/if}

    </div>

    {#if isOwner}
      <div class="flex items-center justify-between">
        <button
          type="button"
          disabled={backfilling || !exifKeep}
          onclick={() => showBackfillConfirm = true}
          class="flex items-center gap-2 rounded-xl border border-gray-200 bg-white px-4 py-2 text-md font-medium text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-300 dark:hover:bg-gray-800"
          title={!exifKeep ? m.extract_exif_first() : undefined}
        >
          {#if backfilling}<Spinner size="sm" />{/if}
          {m.extract_exif_again()}
        </button>

        <button
          type="button"
          disabled={!isDirty || saving}
          onclick={save}
          class="flex items-center gap-2 rounded-xl bg-indigo-600 px-5 py-2 text-md font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
        >
          {#if saving}<Spinner size="sm" />{/if}
          {m.save()}
        </button>
      </div>
    {/if}

  </div>
</div>

{#if showBackfillConfirm}
  <Modal onclose={() => showBackfillConfirm = false}>
    <h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-3">{m.extract_exif_again()}?</h2>
    <p class="text-md text-gray-600 dark:text-gray-400">
      {m.extract_exif_again_description()}
    </p>
    <div class="mt-6 flex justify-end gap-3">
      <button
        type="button"
        onclick={() => showBackfillConfirm = false}
        class="rounded-xl border border-gray-200 bg-white px-4 py-2 text-md font-medium text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-300"
      >
        {m.cancel()}
      </button>
      <button
        type="button"
        onclick={triggerBackfill}
        class="rounded-xl bg-indigo-600 px-4 py-2 text-md font-medium text-white hover:bg-indigo-700"
      >
        {m.reextract()}
      </button>
    </div>
  </Modal>
{/if}
