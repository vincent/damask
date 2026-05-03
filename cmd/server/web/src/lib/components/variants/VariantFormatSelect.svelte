<script lang="ts">
  interface Props {
    value: string
    options: { value: string; label: string }[]
    label?: string
    id?: string
    onchange: (value: string) => void
  }

  let {
    value,
    options,
    label = 'Format',
    id = 'variant-format',
    onchange,
  }: Props = $props()
</script>

<div>
  <label
    for={id}
    class="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-400"
  >
    {label}
  </label>
  <select
    {id}
    {value}
    class="text-md w-full rounded-lg border border-gray-300 px-3 py-2 focus:border-indigo-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
    onchange={(e) => onchange((e.currentTarget as HTMLSelectElement).value)}
  >
    {#each options as option}
      <option value={option.value}>{option.label}</option>
    {/each}
  </select>
</div>
