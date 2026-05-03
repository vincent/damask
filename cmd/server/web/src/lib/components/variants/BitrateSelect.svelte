<script lang="ts">
  interface Props {
    value?: string
    disabled?: boolean
    label?: string
    id?: string
    onchange: (value: string) => void
  }

  let {
    value = '192k',
    disabled = false,
    label = 'Bitrate',
    id = 'variant-bitrate',
    onchange,
  }: Props = $props()

  const bitrates = ['64k', '96k', '128k', '192k', '256k', '320k']
</script>

<div class:opacity-50={disabled}>
  <label
    for={id}
    class="mb-1 block text-sm font-medium text-gray-600 dark:text-gray-400"
  >
    {label}
  </label>
  <select
    {id}
    {disabled}
    {value}
    class="text-md w-full rounded-lg border border-gray-300 px-3 py-2 focus:border-indigo-400 focus:outline-none disabled:cursor-not-allowed dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
    onchange={(e) => onchange((e.currentTarget as HTMLSelectElement).value)}
  >
    {#each bitrates as bitrate}
      <option value={bitrate}>{bitrate}</option>
    {/each}
  </select>
</div>
