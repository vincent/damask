<script lang="ts">
  import '../app.css'
  import { authStore } from '$lib/stores/auth'
  import type { LayoutData } from './$types'

  let { children, data }: { children: any; data: LayoutData } = $props()

  $effect(() => {
    if (data?.user && data?.workspace && data?.role) {
      authStore.login(data.user, data.workspace, data.role)
    }
  })
</script>

{@render children()}
