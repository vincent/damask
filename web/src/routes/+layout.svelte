<script lang="ts">
  import '../app.css'
  import { authStore } from '$lib/stores/auth.svelte'
  import type { LayoutData } from './$types'

  let { children, data }: { children: any; data: LayoutData } = $props()

  $effect.pre(() => {
    if (data?.user && data?.workspace && data?.role) {
      authStore.login(data.user, data.workspace, data.role)
    }
  })
</script>

{@render children()}
