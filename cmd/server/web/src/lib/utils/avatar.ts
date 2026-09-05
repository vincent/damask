// Deterministic hue from string, used for initials-avatar fallbacks.
const AVATAR_PALETTES: [string, string][] = [
  ['oklch(48% 0.18 264)', 'oklch(92% 0.06 264)'], // indigo
  ['oklch(46% 0.14 182)', 'oklch(92% 0.05 182)'], // teal
  ['oklch(48% 0.18 300)', 'oklch(92% 0.06 300)'], // purple
  ['oklch(48% 0.18 350)', 'oklch(92% 0.05 350)'], // pink
  ['oklch(52% 0.16 60)', 'oklch(93% 0.05 60)'], // amber
  ['oklch(48% 0.16 210)', 'oklch(92% 0.05 210)'], // cyan
  ['oklch(46% 0.15 145)', 'oklch(92% 0.05 145)'], // green
  ['oklch(46% 0.18 25)', 'oklch(92% 0.05 25)'], // red
]

export function avatarColors(name: string): [string, string] {
  let h = 0
  for (let i = 0; i < name.length; i++) h = (h * 31 + name.charCodeAt(i)) >>> 0
  return AVATAR_PALETTES[h % AVATAR_PALETTES.length]
}

export function initialsFor(
  name: string | null | undefined,
  fallback: string
): string {
  return (name ?? fallback)
    .split(' ')
    .map((p) => p[0])
    .slice(0, 2)
    .join('')
    .toUpperCase()
}
