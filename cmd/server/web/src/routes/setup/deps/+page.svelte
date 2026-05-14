<script lang="ts">
  import { goto } from '$app/navigation'
  import { onMount } from 'svelte'
  import type { DepStatus } from '$lib/api'
  import { setupApi } from '$lib/api/setup'
  import DepsBadge from '$lib/components/DepsBadge.svelte'
  import SetupStep from '$lib/components/SetupStep.svelte'
  import Button from '$lib/components/ui/Button.svelte'
  import { m } from '$lib/paraglide/messages'

  let deps = $state<DepStatus[]>([])
  let loading = $state(true)

  async function loadDeps() {
    loading = true
    try {
      deps = await setupApi.deps()
    } finally {
      loading = false
    }
  }

  onMount(() => {
    void loadDeps()
  })

  async function onNext() {
    const missing = deps.filter((dep) => !dep.found)
    if (missing.length > 0) {
      const features = missing
        .flatMap((dep) =>
          dep.features.map((feature) => `${dep.name}: ${feature}`)
        )
        .join('\n')
      const ok = confirm(
        `${m.setup_deps_skip_warning_title()}\n\n${m.setup_deps_skip_warning_body({ features })}`
      )
      if (!ok) return
    }
    await goto('/setup/env')
  }
</script>

<SetupStep
  title={m.setup_deps_title()}
  description={m.setup_deps_description()}
  backHref="/setup/storage"
  {onNext}
>
  <div class="flex justify-end">
    <Button variant="secondary" onclick={loadDeps} {loading}>
      {m.setup_deps_recheck()}
    </Button>
  </div>

  <div class="space-y-4">
    {#each deps as dep}
      <DepsBadge
        name={dep.name}
        binary={dep.binary}
        found={dep.found}
        version={dep.version}
        features={dep.features}
        docsUrl={dep.docsUrl}
        checking={loading}
      />
    {/each}
  </div>
</SetupStep>
