<script lang="ts">
  import { m } from '$lib/paraglide/messages'

  interface Props {
    value?: string
    placeholder?: string
    onchange?: (value: string) => void
    class?: string
  }

  let {
    value = $bindable(''),
    placeholder = m.search_(),
    onchange,
    class: extraClass = '',
  }: Props = $props()
</script>

<div class="relative {extraClass}">
  <svg
    class="pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-gray-400 dark:text-gray-500"
    fill="none"
    viewBox="0 0 24 24"
    stroke="currentColor"
  >
    <path
      stroke-linecap="round"
      stroke-linejoin="round"
      stroke-width="2"
      d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
    />
  </svg>
  <input
    type="search"
    data-search
    {placeholder}
    bind:value
    oninput={(e) => {
      value = e.currentTarget.value
      onchange?.(value)
    }}
    class="text-md dark:focus:bg-gray-750 w-full rounded-lg border border-gray-200 bg-gray-50 py-2 pr-3
      pl-9 placeholder-gray-400 focus:border-indigo-400 focus:bg-white focus:ring-1 focus:ring-indigo-400
      focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500
      dark:focus:border-indigo-500 dark:focus:bg-gray-900"
  />
</div>
