import type { Variant } from '$lib/api'

export function deriveVariantMime(
  variant: Variant,
  fallbackMime: string
): string {
  const key = variant.storage_key ?? ''
  if (key.endsWith('.mp4') || key.endsWith('.m4v')) return 'video/mp4'
  if (key.endsWith('.webm')) return 'video/webm'
  if (key.endsWith('.mov')) return 'video/quicktime'
  if (key.endsWith('.mp3')) return 'audio/mpeg'
  if (key.endsWith('.aac')) return 'audio/aac'
  if (key.endsWith('.ogg')) return 'audio/ogg'
  if (key.endsWith('.wav')) return 'audio/wav'
  if (key.endsWith('.flac')) return 'audio/flac'
  if (key.endsWith('.pdf')) return 'application/pdf'
  if (key.endsWith('.png')) return 'image/png'
  if (key.endsWith('.jpg') || key.endsWith('.jpeg')) return 'image/jpeg'
  if (key.endsWith('.webp')) return 'image/webp'
  if (key.endsWith('.gif')) return 'image/gif'
  if (key.endsWith('.avif')) return 'image/avif'
  const t = variant.type ?? ''
  if (t.startsWith('video_'))
    return fallbackMime.startsWith('video/') ? fallbackMime : 'video/mp4'
  if (t.startsWith('audio_'))
    return fallbackMime.startsWith('audio/') ? fallbackMime : 'audio/mpeg'
  if (t === 'video_capture_image') return 'image/jpeg'
  return fallbackMime || 'application/octet-stream'
}
