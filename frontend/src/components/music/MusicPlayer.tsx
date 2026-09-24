import { useEffect, useRef, useState } from 'react'
import { useQuery } from '@apollo/client/react'
import { GET_PLAYLISTS, type Song } from '../../graphql/queries'
import { videoLength } from '../../format'
import { useStoredState } from '../../lib/storage'
import { ListIcon, NextIcon, PauseIcon, PlayIcon, PrevIcon, VolumeIcon } from './icons'
import MusicLibrary from './MusicLibrary'
import { useCrossfadePlayer } from './useCrossfadePlayer'

// Songs crossfade over this long when one ends on its own, and faster when skipped.
const AUTO_FADE_SECONDS = 5
const SKIP_FADE_MS = 1500

// YouTube error codes: 2 bad ID, 5 HTML5 error, 100 not found or private, 101/150 embedding disabled.
function describeError(code: number): string {
  if (code === 101 || code === 150) return "the owner doesn't allow it to be played outside YouTube"
  if (code === 100) return 'it was removed or made private'
  return `YouTube error ${code}`
}

/**
 * A floating music player pinned to the bottom-right. It plays one playlist at a time,
 * shuffling without repeats until every song has played, and crossfades between songs.
 * YouTube requires the player to stay visible, so the current video shows at the top.
 */
export default function MusicPlayer() {
  const { data } = useQuery(GET_PLAYLISTS)
  const playlists = data?.playlists ?? []

  const [current, setCurrent] = useState<Song | null>(null)
  // The chosen playlist, remembered across reloads. Until one is chosen, fall back to the
  // playing song's playlist, then the first one; playSong saves whichever is actually played.
  const [playlistId, setPlaylistId] = useStoredState<number | null>('music-playlist', null)
  const playlist =
    playlists.find((p) => p.id === playlistId) ??
    playlists.find((p) => p.id === current?.playlistId) ??
    playlists[0] ??
    null
  const songs = playlist?.songs ?? []

  const [volume, setVolume] = useStoredState('music-volume', 60)
  const [libraryOpen, setLibraryOpen] = useState(false)
  const [notice, setNotice] = useState<string | null>(null)

  // Songs already played this round, and the order they played in (for the back button).
  const played = useRef(new Set<number>())
  const history = useRef<Song[]>([])
  const [canGoBack, setCanGoBack] = useState(false)

  // Player callbacks fire long after render, so they read the latest state through refs.
  const songsRef = useRef(songs)
  const currentRef = useRef(current)
  useEffect(() => {
    songsRef.current = songs
    currentRef.current = current
  })

  const pickNext = (): Song | null => {
    const all = songsRef.current
    const now = currentRef.current
    let pool = all.filter((s) => s.id !== now?.id && !played.current.has(s.id))
    if (pool.length === 0) {
      // Every song has played: start a new round, still avoiding an immediate repeat.
      played.current.clear()
      pool = all.filter((s) => s.id !== now?.id)
    }
    if (pool.length === 0) return all[0] ?? null
    return pool[Math.floor(Math.random() * pool.length)]
  }

  const playSong = (song: Song, fadeMs: number, remember = true) => {
    if (remember && currentRef.current) history.current.push(currentRef.current)
    setCanGoBack(history.current.length > 0)
    played.current.add(song.id)
    currentRef.current = song
    setCurrent(song)
    if (song.playlistId !== playlistId) setPlaylistId(song.playlistId)
    engine.crossfadeTo(song.videoId, fadeMs)
  }

  const next = (fadeMs: number) => {
    const song = pickNext()
    if (song) playSong(song, fadeMs)
  }

  const engine = useCrossfadePlayer({
    volume,
    fadeSeconds: AUTO_FADE_SECONDS,
    onNearEnd: () => next(AUTO_FADE_SECONDS * 1000),
    onError: (videoId, code) => {
      const song = songsRef.current.find((s) => s.videoId === videoId)
      setNotice(`Skipped “${song?.title ?? videoId}”: ${describeError(code)}.`)
      next(SKIP_FADE_MS)
    },
  })

  // Before anything plays, show a random song so the play button has something to start.
  // The seed keeps that pick stable across renders.
  const [seed] = useState(Math.random)
  const upNext = current ? null : (songs[Math.floor(seed * songs.length)] ?? null)
  const shown = current ?? upNext

  const { ready, cue } = engine
  useEffect(() => {
    if (ready && upNext) cue(upNext.videoId)
  }, [ready, cue, upNext])

  // Hide the "skipped" notice after a while.
  useEffect(() => {
    if (!notice) return
    const id = setTimeout(() => setNotice(null), 6000)
    return () => clearTimeout(id)
  }, [notice])

  // selectPlaylist makes another playlist the one being shuffled, starting a fresh round.
  const selectPlaylist = (id: number) => {
    setPlaylistId(id)
    played.current.clear()
    history.current = []
    setCanGoBack(false)
    songsRef.current = playlists.find((p) => p.id === id)?.songs ?? []
    currentRef.current = null
  }

  // switchPlaylist changes playlist from the picker. While music is playing it crossfades
  // straight into a random song from the new playlist; otherwise it just shows one.
  const switchPlaylist = (id: number) => {
    selectPlaylist(id)
    if (engine.playing && songsRef.current.length > 0) next(SKIP_FADE_MS)
    else setCurrent(null)
  }

  // playFromLibrary plays a song picked in the library, switching to its playlist if needed.
  const playFromLibrary = (song: Song) => {
    if (song.playlistId !== playlist?.id) selectPlaylist(song.playlistId)
    playSong(song, SKIP_FADE_MS)
  }

  const togglePlay = () => {
    if (engine.playing) engine.pause()
    else if (current) engine.resume()
    else if (upNext) playSong(upNext, 800)
  }

  const previous = () => {
    const song = history.current.pop()
    if (song) playSong(song, SKIP_FADE_MS, false)
  }

  const empty = songs.length === 0

  return (
    <aside className="music-player" aria-label="Music player">
      {libraryOpen && (
        <MusicLibrary
          playlists={playlists}
          initialPlaylistId={playlist?.id ?? null}
          currentSongId={current?.id ?? null}
          onPlay={playFromLibrary}
          onClose={() => setLibraryOpen(false)}
        />
      )}

      <div className="music-card">
        {/* Both players stay mounted; the active one is shown and the other fades out. */}
        <div className="music-video">
          <div ref={engine.hostRef(0)} className={engine.active === 0 ? 'music-slot visible' : 'music-slot'} />
          <div ref={engine.hostRef(1)} className={engine.active === 1 ? 'music-slot visible' : 'music-slot'} />
          {empty && (
            <button className="music-empty" onClick={() => setLibraryOpen(true)}>
              {playlists.length === 0 ? 'Create a playlist to start' : 'Add songs to this playlist'}
            </button>
          )}
        </div>

        <div className="music-info">
          {playlists.length > 0 && (
            <select
              className="music-playlist"
              value={playlist?.id}
              onChange={(e) => switchPlaylist(Number(e.target.value))}
              aria-label="Playlist"
            >
              {playlists.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name} ({p.songs.length})
                </option>
              ))}
            </select>
          )}
          <div className="music-title" title={shown?.title}>
            {shown?.title ?? (empty ? 'No songs yet' : 'Loading…')}
          </div>
          <div className="music-channel muted">{shown?.channelName ?? ' '}</div>
          {notice && <div className="music-notice">{notice}</div>}
        </div>

        <SeekBar current={engine.progress.current} duration={engine.progress.duration} onSeek={engine.seekTo} />

        <div className="music-controls">
          <button className="icon-button" onClick={previous} disabled={!canGoBack} aria-label="Previous song">
            <PrevIcon />
          </button>
          <button
            className="icon-button music-play"
            onClick={togglePlay}
            disabled={!ready || empty}
            aria-label={engine.playing ? 'Pause' : 'Play'}
          >
            {engine.playing ? <PauseIcon /> : <PlayIcon />}
          </button>
          <button
            className="icon-button"
            onClick={() => next(SKIP_FADE_MS)}
            disabled={!ready || songs.length < 2}
            aria-label="Next song"
          >
            <NextIcon />
          </button>

          <label className="music-volume" title={`Volume ${volume}%`}>
            <VolumeIcon />
            <input
              type="range"
              min={0}
              max={100}
              value={volume}
              onChange={(e) => setVolume(Number(e.target.value))}
              aria-label="Volume"
            />
          </label>

          <button
            className="icon-button"
            onClick={() => setLibraryOpen(true)}
            aria-label="Open music library"
            title="Music library"
          >
            <ListIcon />
          </button>
        </div>
      </div>
    </aside>
  )
}

// SeekBar shows the song's position and jumps when clicked or dragged. While dragging, it
// shows the drag position and only seeks on release, so YouTube isn't sent a seek per pixel.
function SeekBar({ current, duration, onSeek }: { current: number; duration: number; onSeek: (s: number) => void }) {
  const [drag, setDrag] = useState<number | null>(null)
  const total = Math.floor(duration)
  const value = Math.min(Math.floor(drag ?? current), total)

  const commit = () => {
    if (drag === null) return
    onSeek(drag)
    setDrag(null)
  }

  return (
    <div className="music-seek">
      <span>{videoLength(value)}</span>
      <input
        type="range"
        min={0}
        max={Math.max(total, 1)}
        step={1}
        value={value}
        disabled={total === 0}
        onChange={(e) => setDrag(Number(e.target.value))}
        onPointerUp={commit}
        onKeyUp={commit}
        onBlur={commit}
        aria-label="Seek"
        aria-valuetext={`${videoLength(value)} of ${videoLength(total)}`}
      />
      <span>{total > 0 ? videoLength(total) : '--:--'}</span>
    </div>
  )
}
