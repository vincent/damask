<script lang="ts">
  import { goto } from '$app/navigation'
  import { authApi, ApiError } from '$lib/api/client'

  let name = $state('')
  let email = $state('')
  let password = $state('')
  let error = $state('')
  let loading = $state(false)

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault()
    error = ''
    loading = true

    try {
      await authApi.register(name, email, password)
      goto('/library')
    } catch (err) {
      error = err instanceof ApiError ? err.message : 'Registration failed'
    } finally {
      loading = false
    }
  }
</script>

<svelte:head>
  <title>Create account — Creativo DAM</title>
</svelte:head>

<div class="min-h-screen flex items-center justify-center bg-gray-50">
  <div class="w-full max-w-md space-y-8 p-8 bg-white rounded-xl shadow">
    <div>
      <h1 class="text-2xl font-bold text-gray-900">Create your account</h1>
      <p class="mt-1 text-sm text-gray-600">
        Already have an account? <a href="/login" class="text-blue-600 hover:underline">Sign in</a>
      </p>
    </div>

    <form onsubmit={handleSubmit} class="space-y-4">
      {#if error}
        <p class="text-sm text-red-600 bg-red-50 p-3 rounded">{error}</p>
      {/if}

      <div>
        <label for="name" class="block text-sm font-medium text-gray-700">Full name</label>
        <input
          id="name"
          type="text"
          bind:value={name}
          required
          autocomplete="name"
          class="mt-1 w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
      </div>

      <div>
        <label for="email" class="block text-sm font-medium text-gray-700">Email</label>
        <input
          id="email"
          type="email"
          bind:value={email}
          required
          autocomplete="email"
          class="mt-1 w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
      </div>

      <div>
        <label for="password" class="block text-sm font-medium text-gray-700">Password</label>
        <input
          id="password"
          type="password"
          bind:value={password}
          required
          minlength="8"
          autocomplete="new-password"
          class="mt-1 w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
        <p class="mt-1 text-xs text-gray-500">Minimum 8 characters</p>
      </div>

      <button
        type="submit"
        disabled={loading}
        class="w-full py-2 px-4 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 font-medium"
      >
        {loading ? 'Creating account…' : 'Create account'}
      </button>
    </form>
  </div>
</div>
