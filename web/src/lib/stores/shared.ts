export type CategoryKey = 'image' | 'video' | 'audio' | 'document'

export const CATEGORY_ORDER: CategoryKey[] = ['image', 'video', 'audio', 'document']

export const CATEGORY_LABELS: Record<CategoryKey, string> = {
    image: 'Images & Graphics',
    video: 'Video Production',
    audio: 'Audio & Music',
    document: 'Documents',
}

export const CATEGORY_ICON_BG: Record<CategoryKey, { light: string; dark: string }> = {
    image: { light: 'bg-violet-100 text-violet-600', dark: 'dark:bg-violet-900/50 dark:text-violet-300' },
    video: { light: 'bg-red-100 text-red-600', dark: 'dark:bg-red-900/50 dark:text-red-300' },
    audio: { light: 'bg-emerald-100 text-emerald-600', dark: 'dark:bg-emerald-900/50 dark:text-emerald-300' },
    document: { light: 'bg-blue-100 text-blue-600', dark: 'dark:bg-blue-900/50 dark:text-blue-300' },
}

export const CATEGORY_BORDER: Record<CategoryKey, string> = {
    image: 'border-violet-200 border-violet-700',
    video: 'border-red-200 dark:border-red-700',
    audio: 'border-emerald-200 border-emerald-700',
    document: 'border-blue-200 border-blue-700',
}

export function getProjectColor(project?: { color?: { Valid: boolean; String: string } } | null): string {
    return project?.color?.Valid ? project.color.String : '#9ca3af'
}
