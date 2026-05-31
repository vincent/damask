<script lang="ts">
  import '../app.css'
  import { configStore } from '$lib/stores/config.svelte'
  import { authStore } from '$lib/stores/auth.svelte'
  import Toast from '$lib/components/ui/Toast.svelte'
  import ShortcutHelp from '$lib/components/shortcuts/ShortcutHelp.svelte'
  import ActionSheet from '$lib/components/ActionSheet.svelte'
  import { keymap } from '$lib/shortcuts/store.svelte'
  import { useShortcuts } from '$lib/shortcuts/context'
  import { undoStore } from '$lib/stores/undo.svelte'
  import { themeStore } from '$lib/stores/theme.svelte'
  import type { LayoutData } from './$types'

  let { children, data }: { children: any; data: LayoutData } = $props()

  // Referencing the store ensures the module loads and $effect.root fires on first render.
  keymap.current

  useShortcuts({
    'history.undo': () => {
      undoStore.undo()
    },
    'history.redo': () => {
      undoStore.redo()
    },
  })

  $effect.pre(() => {
    if (data?.user && data?.workspace && data?.role) {
      authStore.login(
        data.user,
        data.workspace,
        data.role,
        data.totalAssetCount ?? 0
      )
    }
    configStore.load()
  })

  $effect(() => {
    themeStore.init()
  })
</script>

<ActionSheet>
  {#snippet children()}
    {@render children?.()}
    <Toast />
    <ShortcutHelp />
  {/snippet}
</ActionSheet>
