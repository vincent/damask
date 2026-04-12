<script lang="ts">
    import Hint from "./Hint.svelte"
    import Title from "./Title.svelte"

    let { title, description, children = undefined } = $props();
</script>

<!-- Header -->
<header class="border-b border-gray-100 bg-white px-6 py-4 dark:border-gray-800 dark:bg-gray-900">
    <div class="flex items-center justify-between">
        <div>
            <Title>{title}</Title>
            <Hint>{description}</Hint>
        </div>
        {@render children?.()}
    </div>
</header>
