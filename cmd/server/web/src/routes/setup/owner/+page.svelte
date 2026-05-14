<script lang="ts">
  import { goto } from '$app/navigation'
  import Input from '$lib/components/ui/Input.svelte'
  import SetupStep from '$lib/components/SetupStep.svelte'
  import { m } from '$lib/paraglide/messages'
  import { setupApi } from '$lib/api/setup'

  let workspaceName = $state('')
  let name = $state('')
  let email = $state('')
  let password = $state('')
  let confirmPassword = $state('')
  let error = $state('')
  let loading = $state(false)

  const strength = $derived.by(() => {
    const charset =
      (/[a-z]/.test(password) ? 26 : 0) +
      (/[A-Z]/.test(password) ? 26 : 0) +
      (/[0-9]/.test(password) ? 10 : 0) +
      (/[^A-Za-z0-9]/.test(password) ? 32 : 0)
    if (!password || charset === 0) return 0
    return Math.min(
      100,
      Math.round(password.length * Math.log2(charset) * 1.35)
    )
  })

  const strengthLabel = $derived(
    strength < 30
      ? 'Weak'
      : strength < 60
        ? 'Fair'
        : strength < 80
          ? 'Strong'
          : 'Excellent'
  )

  async function onNext() {
    if (password !== confirmPassword) {
      error = m.setup_owner_passwords_mismatch()
      return
    }

    loading = true
    error = ''
    try {
      await setupApi.createOwner({ workspaceName, name, email, password })
      await goto('/library')
    } catch (err) {
      error = err instanceof Error ? err.message : m.try_again()
    } finally {
      loading = false
    }
  }
</script>

<SetupStep
  title={m.setup_owner_title()}
  backHref="/setup/env"
  {loading}
  {onNext}
  nextLabel={m.create_account()}
>
  <div class="grid gap-4 md:grid-cols-2">
    <Input
      label={m.setup_owner_workspace()}
      value={workspaceName}
      {error}
      oninput={(e) =>
        (workspaceName = (e.currentTarget as HTMLInputElement).value)}
    />
    <Input
      label={m.setup_owner_name()}
      value={name}
      oninput={(e) => (name = (e.currentTarget as HTMLInputElement).value)}
    />
    <Input
      label={m.setup_owner_email()}
      value={email}
      oninput={(e) => (email = (e.currentTarget as HTMLInputElement).value)}
    />
    <div class="space-y-2">
      <Input
        label={m.setup_owner_password()}
        type="password"
        value={password}
        oninput={(e) =>
          (password = (e.currentTarget as HTMLInputElement).value)}
      />
      <div class="overflow-hidden rounded-full bg-slate-200 dark:bg-slate-800">
        <div
          class="h-2 rounded-full bg-cyan-500 transition-all duration-200"
          style={`width:${strength}%`}
        ></div>
      </div>
      <p class="text-xs text-slate-500 dark:text-slate-400">
        {strengthLabel} · {m.setup_owner_password_hint()}
      </p>
    </div>
    <Input
      label={m.setup_owner_confirm()}
      type="password"
      value={confirmPassword}
      oninput={(e) =>
        (confirmPassword = (e.currentTarget as HTMLInputElement).value)}
    />
  </div>
</SetupStep>
