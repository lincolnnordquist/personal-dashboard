import { useMemo, useState } from 'react'
import { useQuery } from '@apollo/client/react'
import { GET_BACKGROUND_THEMES, GET_WIDGETS } from './graphql/queries'
import Background, { type Scene } from './components/Background'
import Grid from './components/Grid'
import MusicPlayer from './components/music/MusicPlayer'
import SearchBar from './components/SearchBar'
import { greeting } from './format'

export default function App() {
  const { data, loading, error } = useQuery(GET_WIDGETS)
  const { data: themeData } = useQuery(GET_BACKGROUND_THEMES)

  // The selected playlist's background theme, reported by the music player once playlists
  // load. undefined means not known yet; null mixes all themes.
  const [theme, setTheme] = useState<string | null | undefined>(undefined)
  const scenes = useMemo((): Scene[] | null => {
    const themes = themeData?.backgroundThemes
    if (!themes || theme === undefined) return null
    // A theme folder missing on this machine falls back to the mix too.
    const chosen = themes.filter((t) => t.name === theme)
    return (chosen.length > 0 ? chosen : themes).flatMap((t) => t.videos)
  }, [themeData, theme])

  return (
    <>
      <Background scenes={scenes} />
      <main className="app">
        <SearchBar />
        <header className="app-header">
          <h1 className="greeting">{greeting('Lincoln')}</h1>
        </header>
        {loading && <p className="muted">Loading widgets…</p>}
        {error && <p className="error">Could not load widgets: {error.message}</p>}
        {data && <Grid widgets={data.widgets.filter((w) => w.enabled)} />}
      </main>
      <MusicPlayer onThemeChange={setTheme} />
    </>
  )
}
