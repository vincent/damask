type CategoryKey = 'image' | 'video' | 'audio' | 'document'

export const CATEGORY_ORDER: CategoryKey[] = ['image', 'video', 'audio', 'document']

export const CATEGORY_LABELS: Record<CategoryKey, string> = {
    image: 'Images & Graphics',
    video: 'Video Production',
    audio: 'Audio & Music',
    document: 'Documents',
}

export const CATEGORY_ICON_BG: Record<CategoryKey, string> = {
    image: 'bg-violet-100 text-violet-600',
    video: 'bg-red-100 text-red-600',
    audio: 'bg-emerald-100 text-emerald-600',
    document: 'bg-blue-100 text-blue-600',
}

export const CATEGORY_BORDER: Record<CategoryKey, string> = {
    image: 'border-violet-200',
    video: 'border-red-200',
    audio: 'border-emerald-200',
    document: 'border-blue-200',
}

