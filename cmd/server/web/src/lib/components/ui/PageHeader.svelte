<script lang="ts">
    let { title, description, children = undefined } = $props();
</script>

<!-- Header -->
<header class="border-b border-gray-100 bg-white px-6 py-4 dark:border-gray-800 dark:bg-gray-900">
    <div class="flex items-center justify-between">
        <div>
            <h1 class="text-2xl font-bold text-gray-900 dark:text-gray-50">{title}</h1>
            <p class="mt-0.5 text-md text-gray-400">{description}</p>
        </div>
        {@render children?.()}
    </div>
</header>
