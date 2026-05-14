<script lang="ts">
  import { goto } from '$app/navigation'
  import { onDestroy } from 'svelte'
  import type { StorageParams } from '$lib/api'
  import { setupApi } from '$lib/api/setup'
  import SetupStep from '$lib/components/SetupStep.svelte'
  import StorageForm from '$lib/components/StorageForm.svelte'
  import { m } from '$lib/paraglide/messages'
  import { defaultStorageParams, wizardStore } from '$lib/stores/setupWizard'

  let storage = $state<StorageParams>(defaultStorageParams())
  let error = $state('')
  let loading = $state(false)

  const unsubscribe = wizardStore.subscribe((state) => {
    storage = state.storage ?? defaultStorageParams()
  })

  onDestroy(unsubscribe)

  async function onNext() {
    loading = true
    error = ''
    try {
      const result = await setupApi.validateStorage(storage)
      if (!result.ok) {
        error = result.reason ?? m.try_again()
        return
      }
      wizardStore.update((state) => ({ ...state, storage }))
      await goto('/setup/deps')
    } finally {
      loading = false
    }
  }
</script>

<SetupStep title={m.setup_storage_title()} {loading} {onNext}>
  <StorageForm
    value={storage}
    onChange={(next) => {
      storage = next
      error = ''
    }}
    validationError={error}
  />
</SetupStep>
