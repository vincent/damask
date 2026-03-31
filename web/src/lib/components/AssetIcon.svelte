<script lang="ts">
    import { File, Image, Music, Play } from "@lucide/svelte"

    type Props = {
        category: "image" | "video" | "audio" | "document",
        class?: string
    }

    let { category, class: extraClass }: Props = $props()

    const cardBg: Record<string, string> = {
        image: 'bg-amber-300 dark:bg-amber-700',
        video: 'bg-rose-300 dark:bg-rose-700',
        audio: 'bg-emerald-300 dark:bg-emerald-700',
        document: 'bg-blue-200 dark:bg-blue-700',
    }
</script>

<div class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg {cardBg[category]} {extraClass}">
    {#if category === 'image'}
        <Image class="h-5 w-5 text-white" />
    {:else if category === 'video'}
        <Play class="h-5 w-5 text-white" />
    {:else if category === 'audio'}
        <Music class="h-5 w-5 text-white" />
    {:else}
        <File class="h-5 w-5 text-white" />
    {/if}
</div>

