<script lang="ts">
  import { workspaceApi } from '$lib/api'
  import { authStore } from '$lib/stores/auth.svelte'
  import { toastStore } from '$lib/stores/toast.svelte'
  import Spinner from '$lib/components/ui/Spinner.svelte'
  import PageHeader from '$lib/components/ui/PageHeader.svelte'
  import { m } from '$lib/paraglide/messages'

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
      const count =
        mode === 'unlimited' ? 0 : Math.max(1, Math.min(50, capValue))
      const updated = await workspaceApi.updateSettings({
        version_retention_count: count,
      })
      authStore.patchWorkspace({
        version_retention_count: updated.version_retention_count,
      })
      toastStore.show(m.history_settings_saved())
    } catch (e) {
      toastStore.show(
        e instanceof Error ? e.message : m.history_settings_failed(),
        'error'
      )
    } finally {
      saving = false
    }
  }
</script>

<svelte:head>
  <title>{m.version_history()} — Damask</title>
</svelte:head>

<div class="flex-1 overflow-y-auto">
  <PageHeader
    title={m.version_history()}
    description={m.version_history_page_description()}
  />
  <div class="mx-auto w-full max-w-4xl space-y-8 px-8 py-10">
    {#if authStore.role !== 'owner'}
      <p
        class="text-md rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400"
      >
        {m.version_history_settins_only_owners()}
      </p>
    {/if}

    <div
      class="space-y-5 rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-700 dark:bg-gray-900"
    >
      <!-- Keep all -->
      <label class="flex cursor-pointer items-start gap-3">
        <input
          type="radio"
          name="retention"
          value="unlimited"
          class="mt-1.5 accent-indigo-600"
          bind:group={mode}
          disabled={authStore.role !== 'owner'}
        />
        <div class="flex-1 space-y-2">
          <p class="text-md font-medium text-gray-900 dark:text-gray-100">
            {m.keep_all_versions()}
          </p>
          <p class="text-sm text-gray-500 dark:text-gray-400">
            {m.keep_all_versions_description()}
          </p>
        </div>
      </label>

      <!-- Cap -->
      <label class="flex cursor-pointer items-start gap-3">
        <input
          type="radio"
          name="retention"
          value="capped"
          class="mt-1.5 accent-indigo-600"
          bind:group={mode}
          disabled={authStore.role !== 'owner'}
        />
        <div class="flex-1 space-y-2">
          <p class="text-md font-medium text-gray-900 dark:text-gray-100">
            {m.keep_n_versions()}
          </p>
          <p class="text-sm text-gray-500 dark:text-gray-400">
            {m.keep_n_versions_description()}
          </p>
          {#if mode === 'capped'}
            <div class="flex items-center gap-3">
              <input
                type="number"
                min="1"
                max="50"
                class="text-md w-24 rounded-xl border border-gray-200 bg-white px-3 py-1.5 text-gray-800 focus:border-indigo-400 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
                bind:value={capValue}
                disabled={authStore.role !== 'owner'}
              />
              <span class="text-sm text-gray-400">{m.keep_n_versions_ex()}</span
              >
            </div>
          {/if}
        </div>
      </label>
    </div>

    {#if authStore.role === 'owner'}
      <div class="flex justify-end">
        <button
          type="button"
          class="text-md flex items-center gap-2 rounded-xl bg-indigo-600 px-5 py-2 font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
          disabled={!isDirty || saving}
          onclick={save}
        >
          {#if saving}<Spinner size="sm" />{/if}
          {m.save()}
        </button>
      </div>
    {/if}
  </div>
</div>
