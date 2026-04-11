<script lang="ts">
  import { workspaceApi } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import Modal from '$lib/components/ui/Modal.svelte'

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
      toastStore.show('Privacy settings saved')
    } catch (e) {
      toastStore.show(e instanceof Error ? e.message : 'Failed to save settings', 'error')
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
  <title>EXIF & Privacy — Damask</title>
</svelte:head>

<div class="flex-1 overflow-y-auto">
  <div class="mx-auto w-full max-w-2xl px-8 py-10 space-y-8">

    <div>
      <h1 class="text-xl font-semibold text-gray-900 dark:text-gray-100">EXIF & Privacy</h1>
      <p class="mt-1 text-md text-gray-500 dark:text-gray-400">Control how photo metadata is extracted and stored. Changes apply to future uploads; use the backfill button to process existing assets.</p>
    </div>

    {#if !isOwner}
      <p class="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-md text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400">
        Only workspace owners can change these settings.
      </p>
    {/if}

    <div class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-700 dark:bg-gray-900 space-y-6">

      <!-- EXIF extraction toggle -->
      <div class="flex items-start justify-between gap-4">
        <div class="flex-1">
          <p class="text-md font-medium text-gray-900 dark:text-gray-100">Extract photo metadata</p>
          <p class="mt-0.5 text-sm text-gray-500 dark:text-gray-400">Automatically extract camera model, shutter speed, ISO, and other technical metadata from uploaded photos. Stored as custom fields on each asset.</p>
        </div>
        <button
          type="button"
          role="switch"
          aria-checked={exifKeep}
          aria-label="Extract photo metadata"
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
            <p class="text-md font-medium text-gray-900 dark:text-gray-100">Retain GPS coordinates</p>
            <p class="mt-0.5 text-sm text-gray-500 dark:text-gray-400">By default, GPS location data is stripped to protect privacy. Enable only if your workflow requires location-tagged assets. Applies to future ingests; use backfill to re-extract existing assets.</p>
          </div>
          <button
            type="button"
            role="switch"
            aria-checked={exifKeepGPS}
            aria-label="Retain GPS coordinates"
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
          title={!exifKeep ? 'Enable EXIF extraction first' : undefined}
        >
          {#if backfilling}<Spinner size="sm" />{/if}
          Re-extract EXIF from all assets
        </button>

        <button
          type="button"
          disabled={!isDirty || saving}
          onclick={save}
          class="flex items-center gap-2 rounded-xl bg-indigo-600 px-5 py-2 text-md font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
        >
          {#if saving}<Spinner size="sm" />{/if}
          Save
        </button>
      </div>
    {/if}

  </div>
</div>

{#if showBackfillConfirm}
  <Modal onclose={() => showBackfillConfirm = false}>
    <h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-3">Re-extract EXIF from all assets?</h2>
    <p class="text-md text-gray-600 dark:text-gray-400">
      This will queue a background job for every image asset that hasn't had EXIF extracted yet.
      The process runs in the background and may take a few minutes for large libraries.
    </p>
    <div class="mt-6 flex justify-end gap-3">
      <button
        type="button"
        onclick={() => showBackfillConfirm = false}
        class="rounded-xl border border-gray-200 bg-white px-4 py-2 text-md font-medium text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-300"
      >
        Cancel
      </button>
      <button
        type="button"
        onclick={triggerBackfill}
        class="rounded-xl bg-indigo-600 px-4 py-2 text-md font-medium text-white hover:bg-indigo-700"
      >
        Re-extract
      </button>
    </div>
  </Modal>
{/if}
