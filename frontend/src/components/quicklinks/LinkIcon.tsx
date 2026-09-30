import { useState } from 'react'
import type { QuickLink } from '../../graphql/queries'

// isImage tells an image URL override apart from an emoji.
function isImage(icon: string): boolean {
  return /^(https?:\/\/|\/|data:image\/)/.test(icon)
}

// LinkIcon shows a link's icon: its override (an emoji or image), else the site's favicon as
// fetched by the dashboard, else the title's first letter when neither loads.
export default function LinkIcon({ link }: { link: Pick<QuickLink, 'title' | 'icon' | 'faviconUrl'> }) {
  const src = link.icon ? (isImage(link.icon) ? link.icon : null) : link.faviconUrl
  // Remember which image failed, so a changed src gets a fresh try.
  const [failed, setFailed] = useState<string | null>(null)

  if (link.icon && !src) {
    return (
      <span className="link-icon link-icon-emoji" aria-hidden="true">
        {link.icon}
      </span>
    )
  }
  if (src && failed !== src) {
    return (
      <span className="link-icon" aria-hidden="true">
        <img src={src} alt="" loading="lazy" onError={() => setFailed(src)} />
      </span>
    )
  }
  return (
    <span className="link-icon link-icon-letter" aria-hidden="true">
      {link.title.trim().charAt(0).toUpperCase() || '?'}
    </span>
  )
}
