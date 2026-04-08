<script lang="ts">
  import { workspaceApi } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'

  type RetentionMode = 'unlimited' | 'capped'

  const ws = $derived(authStore.workspace)

  let mode = $state<RetentionMode>('unlimited')
  let capValue = $state(10)
  let saving = $state(false)

  // Initialise from workspace once available
  $effect(() => {
    if (ws) {
      if (ws.version_retention_count > 0) {
        mode = 'capped'
        capValue = ws.version_retention_count
      } else {
        mode = 'unlimited'
      }
    }
  })

  const isDirty = $derived(
    ws
      ? mode === 'unlimited'
        ? ws.version_retention_count !== 0
        : ws.version_retention_count !== capValue
      : false
  )

  async function save() {
    saving = true
    try {
      const count = mode === 'unlimited' ? 0 : Math.max(1, Math.min(50, capValue))
      const updated = await workspaceApi.updateSettings({ version_retention_count: count })
      authStore.patchWorkspace({ version_retention_count: updated.version_retention_count })
      toastStore.show('Version history settings saved')
    } catch (e) {
      toastStore.show(e instanceof Error ? e.message : 'Failed to save settings', 'error')
    } finally {
      saving = false
    }
  }
</script>

<svelte:head>
  <title>Version History — Damask</title>
</svelte:head>

<div class="flex flex-1 flex-col overflow-y-auto bg-gray-50 dark:bg-gray-950">
  <div class="mx-auto w-full max-w-2xl px-8 py-10 space-y-8">

    <div>
      <h1 class="text-xl font-semibold text-gray-900 dark:text-gray-100">Version History</h1>
      <p class="mt-1 text-md text-gray-500 dark:text-gray-400">Control how many versions are kept per asset. Versions beyond the limit are soft-deleted during the nightly cleanup and permanently purged after 7 days.</p>
    </div>

    {#if authStore.role !== 'owner'}
      <p class="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-md text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400">
        Only workspace owners can change version history settings.
      </p>
    {/if}

    <div class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-700 dark:bg-gray-900 space-y-5">

      <!-- Keep all -->
      <label class="flex cursor-pointer items-start gap-3">
        <input
          type="radio"
          name="retention"
          value="unlimited"
          class="mt-0.5 accent-indigo-600"
          bind:group={mode}
          disabled={authStore.role !== 'owner'}
        />
        <div>
          <p class="text-md font-medium text-gray-900 dark:text-gray-100">Keep all versions</p>
          <p class="text-sm text-gray-500 dark:text-gray-400">No limit — every uploaded version is preserved indefinitely.</p>
        </div>
      </label>

      <!-- Cap -->
      <label class="flex cursor-pointer items-start gap-3">
        <input
          type="radio"
          name="retention"
          value="capped"
          class="mt-0.5 accent-indigo-600"
          bind:group={mode}
          disabled={authStore.role !== 'owner'}
        />
        <div class="flex-1 space-y-2">
          <p class="text-md font-medium text-gray-900 dark:text-gray-100">Keep last N versions per asset</p>
          <p class="text-sm text-gray-500 dark:text-gray-400">Versions beyond the limit are removed during the nightly cleanup.</p>
          {#if mode === 'capped'}
            <div class="flex items-center gap-3">
              <input
                type="number"
                min="1"
                max="50"
                class="w-24 rounded-xl border border-gray-200 bg-white px-3 py-1.5 text-md text-gray-800 focus:border-indigo-400 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
                bind:value={capValue}
                disabled={authStore.role !== 'owner'}
              />
              <span class="text-sm text-gray-400">versions (1–50)</span>
            </div>
          {/if}
        </div>
      </label>
    </div>

    {#if authStore.role === 'owner'}
      <div class="flex justify-end">
        <button
          type="button"
          class="flex items-center gap-2 rounded-xl bg-indigo-600 px-5 py-2 text-md font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
          disabled={!isDirty || saving}
          onclick={save}
        >
          {#if saving}<Spinner size="sm" />{/if}
          Save
        </button>
      </div>
    {/if}

  </div>
</div>
