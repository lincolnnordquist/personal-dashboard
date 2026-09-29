import { useEffect } from 'react'

/**
 * Loads a background theme's font and applies it to the whole page by setting the
 * --theme-font CSS variable, which index.css puts first in every font stack. With no theme or
 * no font file, the variable is removed and the page falls back to its default fonts.
 */
export function useThemeFont(theme: string | null, fontUrl: string | null) {
  useEffect(() => {
    const root = document.documentElement
    if (!theme || !fontUrl) {
      root.style.removeProperty('--theme-font')
      return
    }
    // One font family per theme, so switching back to a theme reuses its loaded font.
    const family = `theme-${theme.toLowerCase().replace(/[^a-z0-9]+/g, '-')}`
    const face = new FontFace(family, `url("${fontUrl}")`)
    let current = true
    face
      .load()
      .then((loaded) => {
        if (!current) return
        document.fonts.add(loaded)
        root.style.setProperty('--theme-font', `"${family}"`)
      })
      .catch(() => {
        // An unreadable font file: keep the default fonts.
        if (current) root.style.removeProperty('--theme-font')
      })
    // The previous font stays applied until the next one has loaded, so switching themes
    // doesn't flash the default font in between.
    return () => {
      current = false
    }
  }, [theme, fontUrl])
}
