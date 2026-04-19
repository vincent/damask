<script lang="ts">
  import '../app.css'
  import { configStore } from '$lib/stores/config.svelte'
  import { authStore } from '$lib/stores/auth.svelte'
  import Toast from '$lib/components/ui/Toast.svelte'
  import ShortcutHelp from '$lib/components/shortcuts/ShortcutHelp.svelte'
  import { keymap } from '$lib/stores/shortcuts'
  import type { LayoutData } from './$types'

  let { children, data }: { children: any; data: LayoutData } = $props()

  // Referencing the store ensures the module loads and initDispatcher fires on first value.
  $keymap;

  $effect.pre(() => {
    if (data?.user && data?.workspace && data?.role) {
      authStore.login(data.user, data.workspace, data.role)
    }
    configStore.load()
  })
</script>

{@render children()}

<Toast />
<ShortcutHelp />
