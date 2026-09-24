// Small inline icons for the music player (Material Symbols paths, 24×24 viewBox).

function Icon({ d, size = 20 }: { d: string; size?: number }) {
  return (
    <svg viewBox="0 0 24 24" width={size} height={size} aria-hidden="true">
      <path fill="currentColor" d={d} />
    </svg>
  )
}

export const PlayIcon = () => <Icon size={26} d="M8 5v14l11-7z" />
export const PauseIcon = () => <Icon size={26} d="M6 19h4V5H6zm8-14v14h4V5z" />
export const NextIcon = () => <Icon d="M6 18l8.5-6L6 6zM16 6v12h2V6z" />
export const PrevIcon = () => <Icon d="M6 6h2v12H6zm3.5 6 8.5 6V6z" />
export const ListIcon = () => <Icon d="M3 13h2v-2H3zm0 4h2v-2H3zm0-8h2V7H3zm4 4h14v-2H7zm0 4h14v-2H7zM7 7v2h14V7z" />
export const VolumeIcon = () => (
  <Icon
    size={18}
    d="M3 9v6h4l5 5V4L7 9zm13.5 3A4.5 4.5 0 0 0 14 7.97v8.05c1.48-.73 2.5-2.25 2.5-4.02M14 3.23v2.06c2.89.86 5 3.54 5 6.71s-2.11 5.85-5 6.71v2.06c4.01-.91 7-4.49 7-8.77s-2.99-7.86-7-8.77"
  />
)
export const CloseIcon = () => (
  <Icon size={16} d="M19 6.41 17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z" />
)
