<script lang="ts">
  import { authStore } from '$lib/stores/auth'
</script>

<svelte:head>
  <title>Library — Creativo DAM</title>
</svelte:head>

<div class="flex h-screen bg-gray-100">
  <!-- Sidebar placeholder -->
  <aside class="w-64 bg-white border-r border-gray-200 p-4">
    <div class="flex items-center justify-between mb-6">
      <h2 class="font-semibold text-gray-900">Creativo</h2>
      <a href="/logout" class="text-xs text-gray-500 hover:text-gray-700">Sign out</a>
    </div>

    {#if $authStore.workspace}
      <p class="text-sm text-gray-600 mb-4">{$authStore.workspace.name}</p>
    {/if}

    <nav class="space-y-1">
      <a
        href="/library"
        class="flex items-center px-3 py-2 text-sm font-medium rounded-md bg-blue-50 text-blue-700"
      >
        All assets
      </a>
    </nav>
  </aside>

  <!-- Main content -->
  <main class="flex-1 p-6 overflow-auto">
    <div class="mb-6">
      <h1 class="text-xl font-semibold text-gray-900">Library</h1>
      <p class="text-sm text-gray-500 mt-1">Asset grid coming in Phase 2</p>
    </div>

    <div class="flex items-center justify-center h-64 border-2 border-dashed border-gray-300 rounded-lg">
      <p class="text-gray-400">Drop files here to upload</p>
    </div>
  </main>
</div>
