export type CategoryKey = 'image' | 'video' | 'audio' | 'document'

export const CATEGORY_ORDER: CategoryKey[] = ['image', 'video', 'audio', 'document']

export const CATEGORY_LABELS: Record<CategoryKey, string> = {
    image: 'Images & Graphics',
    video: 'Video Production',
    audio: 'Audio & Music',
    document: 'Documents',
}

const base = {
    image: 'sky',
    video: 'red',
    audio: 'emerald',
    document: 'blue',
}

export const ASSET_BACKGROUND_COLORS: Record<string, string> = {
    image: `bg-${base.image}-300 dark:bg-${base.image}-700`,
    video: `bg-${base.video}-300 dark:bg-${base.video}-700`,
    audio: `bg-${base.audio}-300 dark:bg-${base.audio}-700`,
    document: `bg-${base.document}-200 dark:bg-${base.document}-700`,
}

export const DOT_COLORS: Record<string, string> = {
    image: `bg-${base.image}-200 dark:bg-${base.image}-600`,
    video: `bg-${base.video}-200 dark:bg-${base.video}-600`,
    audio: `bg-${base.audio}-200 dark:bg-${base.audio}-600`,
    document: `bg-${base.document}-100 dark:bg-${base.document}-600`,
}

export const CATEGORY_ICON_BG: Record<CategoryKey, { light: string; dark: string }> = {
    image: { light: `bg-${base.image}-100 text-${base.image}-600`, dark: `dark:bg-${base.image}-900/50 dark:text-${base.image}-300` },
    video: { light: `bg-${base.video}-100 text-${base.video}-600`, dark: `dark:bg-${base.video}-900/50 dark:text-${base.video}-300` },
    audio: { light: `bg-${base.audio}-100 text-${base.audio}-600`, dark: `dark:bg-${base.audio}-900/50 dark:text-${base.audio}-300` },
    document: { light: `bg-${base.document}-100 text-${base.document}-600`, dark: `dark:bg-${base.document}-900/50 dark:text-${base.document}-300` },
}

export const CATEGORY_BORDER: Record<CategoryKey, string> = {
    image: `border-${base.image}-200 dark:border-${base.image}-700`,
    video: `border-${base.video}-200 dark:border-${base.video}-700`,
    audio: `border-${base.audio}-200 dark:border-${base.audio}-700`,
    document: `border-${base.document}-200 dark:border-${base.document}-700`,
}

export function getProjectColor(project?: { color?: string | null } | null, opacity = 'ff'): string {
    return `${project?.color ?? '#9ca3af'}${opacity}`
}
