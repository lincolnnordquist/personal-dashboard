import { useEffect, useState, type FormEvent } from 'react'
import { createPortal } from 'react-dom'
import { useMutation, useQuery } from '@apollo/client/react'
import {
  ADD_SONG,
  CREATE_PLAYLIST,
  DELETE_PLAYLIST,
  GET_BACKGROUND_THEMES,
  GET_PLAYLISTS,
  REMOVE_SONG,
  RENAME_PLAYLIST,
  SET_PLAYLIST_THEME,
  type Playlist,
  type Song,
} from '../../graphql/queries'
import { CloseIcon } from './icons'

const refetchQueries = [GET_PLAYLISTS]

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

// MusicLibrary is a centered window for managing playlists: playlists on the left, the selected
// playlist's songs on the right. Clicking a playlist also selects it in the player (switching
// the background to its theme), and clicking a song plays it.
export default function MusicLibrary({
  playlists,
  initialPlaylistId,
  currentSongId,
  onPlay,
  onSelectPlaylist,
  onClose,
}: {
  playlists: Playlist[]
  initialPlaylistId: number | null
  currentSongId: number | null
  onPlay: (song: Song) => void
  onSelectPlaylist: (id: number) => void
  onClose: () => void
}) {
  const [selectedId, setSelectedId] = useState(initialPlaylistId)
  const selected = playlists.find((p) => p.id === selectedId) ?? playlists[0] ?? null

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  return createPortal(
    <div className="library-backdrop" onClick={onClose}>
      <div
        className="library"
        role="dialog"
        aria-modal="true"
        aria-label="Music library"
        onClick={(e) => e.stopPropagation()}
      >
        <header className="library-header">
          <h2>Music library</h2>
          <button className="icon-button" onClick={onClose} aria-label="Close">
            <CloseIcon />
          </button>
        </header>
        <div className="library-body">
          <PlaylistSidebar
            playlists={playlists}
            selectedId={selected?.id ?? null}
            onSelect={(id) => {
              setSelectedId(id)
              onSelectPlaylist(id)
            }}
            onCreated={setSelectedId}
          />
          {selected ? (
            <PlaylistPanel
              key={selected.id}
              playlist={selected}
              currentSongId={currentSongId}
              onPlay={onPlay}
              onDeleted={() => setSelectedId(null)}
            />
          ) : (
            <p className="muted library-empty">Create a playlist to start adding songs.</p>
          )}
        </div>
      </div>
    </div>,
    document.body,
  )
}

function PlaylistSidebar({
  playlists,
  selectedId,
  onSelect,
  onCreated,
}: {
  playlists: Playlist[]
  selectedId: number | null
  onSelect: (id: number) => void
  // Called with a new playlist, which opens in the library without switching the player.
  onCreated: (id: number) => void
}) {
  const [creating, setCreating] = useState(false)
  const [name, setName] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [createPlaylist, { loading }] = useMutation(CREATE_PLAYLIST, { refetchQueries, awaitRefetchQueries: true })

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return
    try {
      const { data } = await createPlaylist({ variables: { name } })
      setName('')
      setCreating(false)
      setError(null)
      if (data) onCreated(data.createPlaylist.id)
    } catch (err) {
      setError(errorMessage(err))
    }
  }

  return (
    <nav className="library-sidebar" aria-label="Playlists">
      <h3 className="section-label">Playlists</h3>
      <ul>
        {playlists.map((p) => (
          <li key={p.id}>
            <button className={p.id === selectedId ? 'playlist-link active' : 'playlist-link'} onClick={() => onSelect(p.id)}>
              <span className="playlist-link-name">{p.name}</span>
              <span className="muted">{p.songs.length}</span>
            </button>
          </li>
        ))}
      </ul>
      {creating ? (
        <form className="inline-form" onSubmit={onSubmit}>
          <input
            autoFocus
            value={name}
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => e.key === 'Escape' && (e.stopPropagation(), setCreating(false))}
            placeholder="Playlist name"
            aria-label="New playlist name"
            disabled={loading}
          />
          <button type="submit" className="button" disabled={loading || !name.trim()}>
            Create
          </button>
        </form>
      ) : (
        <button className="button button-quiet new-playlist" onClick={() => setCreating(true)}>
          + New playlist
        </button>
      )}
      {error && <p className="error small">{error}</p>}
    </nav>
  )
}

function PlaylistPanel({
  playlist,
  currentSongId,
  onPlay,
  onDeleted,
}: {
  playlist: Playlist
  currentSongId: number | null
  onPlay: (song: Song) => void
  onDeleted: () => void
}) {
  const [mode, setMode] = useState<'view' | 'rename' | 'confirm-delete'>('view')
  const [name, setName] = useState(playlist.name)
  const [url, setUrl] = useState('')
  const [message, setMessage] = useState<string | null>(null)

  const [renamePlaylist] = useMutation(RENAME_PLAYLIST, { refetchQueries })
  const [deletePlaylist] = useMutation(DELETE_PLAYLIST, { refetchQueries, awaitRefetchQueries: true })
  const [addSong, { loading: adding }] = useMutation(ADD_SONG, { refetchQueries })
  const [removeSong] = useMutation(REMOVE_SONG, { refetchQueries })
  const [setTheme] = useMutation(SET_PLAYLIST_THEME, { refetchQueries })
  const { data: themeData } = useQuery(GET_BACKGROUND_THEMES)
  const themes = themeData?.backgroundThemes ?? []

  const onThemeChange = async (theme: string) => {
    try {
      await setTheme({ variables: { id: playlist.id, theme: theme || null } })
    } catch (err) {
      setMessage(errorMessage(err))
    }
  }

  const onRename = async (e: FormEvent) => {
    e.preventDefault()
    try {
      await renamePlaylist({ variables: { id: playlist.id, name } })
      setMode('view')
      setMessage(null)
    } catch (err) {
      setMessage(errorMessage(err))
    }
  }

  const onDelete = async () => {
    try {
      await deletePlaylist({ variables: { id: playlist.id } })
      onDeleted()
    } catch (err) {
      setMessage(errorMessage(err))
      setMode('view')
    }
  }

  const onAdd = async (e: FormEvent) => {
    e.preventDefault()
    if (!url.trim()) return
    setMessage(null)
    try {
      const { data } = await addSong({ variables: { playlistId: playlist.id, url } })
      setUrl('')
      if (data) setMessage(`Added “${data.addSong.title}”`)
    } catch (err) {
      setMessage(errorMessage(err))
    }
  }

  const count = playlist.songs.length

  return (
    <section className="library-main">
      <div className="playlist-header">
        {mode === 'rename' ? (
          <form className="inline-form" onSubmit={onRename}>
            <input
              autoFocus
              value={name}
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => e.key === 'Escape' && (e.stopPropagation(), setMode('view'))}
              aria-label="Playlist name"
            />
            <button type="submit" className="button" disabled={!name.trim()}>
              Save
            </button>
            <button type="button" className="button button-quiet" onClick={() => setMode('view')}>
              Cancel
            </button>
          </form>
        ) : mode === 'confirm-delete' ? (
          <div className="confirm">
            <span>
              Delete “{playlist.name}” and its {count} {count === 1 ? 'song' : 'songs'}?
            </span>
            <button className="button button-danger" onClick={onDelete}>
              Delete
            </button>
            <button className="button button-quiet" onClick={() => setMode('view')}>
              Cancel
            </button>
          </div>
        ) : (
          <>
            <div>
              <h3 className="playlist-title">{playlist.name}</h3>
              <span className="muted">
                {count} {count === 1 ? 'song' : 'songs'}
              </span>
            </div>
            <div className="playlist-actions">
              <button className="button button-quiet" onClick={() => (setName(playlist.name), setMode('rename'))}>
                Rename
              </button>
              <button className="button button-quiet" onClick={() => setMode('confirm-delete')}>
                Delete
              </button>
            </div>
          </>
        )}
      </div>

      <label className="theme-picker">
        <span className="muted">Background</span>
        <select value={playlist.theme ?? ''} onChange={(e) => onThemeChange(e.target.value)}>
          <option value="">Mix of all themes</option>
          {themes.map((t) => (
            <option key={t.name} value={t.name}>
              {t.name} ({t.videos.length} {t.videos.length === 1 ? 'video' : 'videos'})
            </option>
          ))}
          {/* A theme set on another machine whose folder isn't here yet. */}
          {playlist.theme && !themes.some((t) => t.name === playlist.theme) && (
            <option value={playlist.theme}>{playlist.theme} (folder missing)</option>
          )}
        </select>
      </label>

      <form className="inline-form song-add" onSubmit={onAdd}>
        <input
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          placeholder="Paste a YouTube link to add it to this playlist"
          aria-label="YouTube link to add"
          disabled={adding}
        />
        <button type="submit" className="button" disabled={adding || !url.trim()}>
          {adding ? 'Adding…' : 'Add'}
        </button>
      </form>
      {message && <p className="muted small">{message}</p>}

      <ul className="song-list">
        {count === 0 && <li className="muted">No songs yet. Paste a YouTube link above.</li>}
        {playlist.songs.map((s) => (
          <li key={s.id} className={s.id === currentSongId ? 'song-item current' : 'song-item'}>
            <button className="song-play" onClick={() => onPlay(s)} title={`Play “${s.title}”`}>
              <img src={s.thumbnailUrl} alt="" loading="lazy" />
              <span className="song-text">
                <span className="song-title">{s.title}</span>
                <span className="song-channel muted">{s.channelName}</span>
              </span>
            </button>
            <button
              className="icon-button"
              onClick={() => removeSong({ variables: { id: s.id } })}
              aria-label={`Remove “${s.title}”`}
              title="Remove from playlist"
            >
              <CloseIcon />
            </button>
          </li>
        ))}
      </ul>
    </section>
  )
}
