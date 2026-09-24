import { useCallback, useEffect, useRef, useState } from 'react'
import { loadYouTubeIframeApi } from '../../lib/youtubeIframeApi'

type Slot = 0 | 1

const FADE_TICK_MS = 50

interface Options {
  // Master volume, 0-100.
  volume: number
  // How long before a song ends to start fading into the next one.
  fadeSeconds: number
  // Called once per song when the active video nears its end, or ends.
  onNearEnd: () => void
  // Called when the active video fails to play, e.g. embedding disabled (codes 101/150).
  onError: (videoId: string | null, code: number) => void
}

/**
 * Two YouTube IFrame players that take turns. The next song loads in the idle player, then the
 * two crossfade by ramping their volumes. Attach `hostRef(0)` and `hostRef(1)` to two stacked
 * elements; each gets an iframe of its own.
 */
export function useCrossfadePlayer({ volume, fadeSeconds, onNearEnd, onError }: Options) {
  const hosts = useRef<[HTMLDivElement | null, HTMLDivElement | null]>([null, null])
  const players = useRef<[YT.Player | null, YT.Player | null]>([null, null])
  const videos = useRef<[string | null, string | null]>([null, null])
  // Each player's fade level, 0-1. Its actual volume is level × master volume.
  const levels = useRef<[number, number]>([0, 0])
  const fadeTimer = useRef<number | null>(null)
  const activeRef = useRef<Slot>(0)
  const playingRef = useRef(false)
  const nearEndFired = useRef(false)

  const [active, setActive] = useState<Slot>(0)
  const [ready, setReady] = useState(false)
  const [playing, setPlaying] = useState(false)
  // Position of the active song, in seconds. duration is 0 until the video has loaded.
  const [progress, setProgress] = useState({ current: 0, duration: 0 })

  // The player's event handlers are created once, so they call the latest props through refs.
  const volumeRef = useRef(volume)
  const onNearEndRef = useRef(onNearEnd)
  const onErrorRef = useRef(onError)
  useEffect(() => {
    onNearEndRef.current = onNearEnd
    onErrorRef.current = onError
  })

  const applyVolume = (slot: Slot) => {
    players.current[slot]?.setVolume(Math.round(levels.current[slot] * volumeRef.current))
  }

  useEffect(() => {
    volumeRef.current = volume
    applyVolume(0)
    applyVolume(1)
  }, [volume])

  // Create both players once. Each goes in a child element created here, not in a React-managed
  // node, because the API replaces its element with an iframe.
  useEffect(() => {
    let cancelled = false
    const created: YT.Player[] = []
    loadYouTubeIframeApi().then((api) => {
      if (cancelled) return
      let readyCount = 0
      for (const slot of [0, 1] as const) {
        const host = hosts.current[slot]
        if (!host) continue
        const el = document.createElement('div')
        host.replaceChildren(el)
        const player = new api.Player(el, {
          width: '100%',
          height: '100%',
          playerVars: { controls: 0, disablekb: 1, fs: 0, iv_load_policy: 3, playsinline: 1, rel: 0 },
          events: {
            onReady: () => {
              player.setVolume(0)
              if (++readyCount === 2) setReady(true)
            },
            onStateChange: (e) => {
              if (e.data === api.PlayerState.ENDED && slot === activeRef.current && !nearEndFired.current) {
                nearEndFired.current = true
                onNearEndRef.current()
              }
            },
            onError: (e) => {
              if (slot === activeRef.current) onErrorRef.current(videos.current[slot], e.data)
            },
          },
        })
        players.current[slot] = player
        created.push(player)
      }
    })
    return () => {
      cancelled = true
      if (fadeTimer.current) clearInterval(fadeTimer.current)
      for (const p of created) p.destroy()
      players.current = [null, null]
    }
  }, [])

  // Track the active song's position, for the seek bar and so the next song can start fading
  // in before this one ends.
  useEffect(() => {
    if (!playing) return
    const id = setInterval(() => {
      const p = players.current[activeRef.current]
      if (!p?.getDuration) return
      const duration = p.getDuration()
      const current = p.getCurrentTime()
      setProgress({ current, duration })
      if (duration > 0 && duration - current <= fadeSeconds && !nearEndFired.current) {
        nearEndFired.current = true
        onNearEndRef.current()
      }
    }, 500)
    return () => clearInterval(id)
  }, [playing, fadeSeconds])

  // fade ramps both players' levels to the targets over ms, then calls done.
  const fade = (targets: [number, number], ms: number, done?: () => void) => {
    if (fadeTimer.current) clearInterval(fadeTimer.current)
    const start: [number, number] = [...levels.current]
    const began = performance.now()
    fadeTimer.current = window.setInterval(() => {
      const k = Math.min(1, (performance.now() - began) / ms)
      for (const slot of [0, 1] as const) {
        levels.current[slot] = start[slot] + (targets[slot] - start[slot]) * k
        applyVolume(slot)
      }
      if (k >= 1) {
        clearInterval(fadeTimer.current!)
        fadeTimer.current = null
        done?.()
      }
    }, FADE_TICK_MS)
  }

  const setActiveSlot = (slot: Slot) => {
    activeRef.current = slot
    setActive(slot)
  }

  // cue shows a video in the active player without playing it. It only touches refs, so it is
  // stable across renders and safe to use as an effect dependency.
  const cue = useCallback((videoId: string) => {
    const slot = activeRef.current
    videos.current[slot] = videoId
    players.current[slot]?.cueVideoById(videoId)
    setProgress({ current: 0, duration: 0 })
  }, [])

  // Player events call these methods through stale closures, so playing is also kept in a ref.
  const setPlayingState = (value: boolean) => {
    playingRef.current = value
    setPlaying(value)
  }

  return {
    ready,
    playing,
    active,
    progress,
    hostRef: (slot: Slot) => (el: HTMLDivElement | null) => {
      hosts.current[slot] = el
    },

    cue,

    // crossfadeTo starts a video in the idle player and fades over to it. With nothing
    // playing, it fades the video in on the active player instead.
    crossfadeTo(videoId: string, ms: number) {
      const from = activeRef.current
      const to: Slot = playingRef.current ? (from === 0 ? 1 : 0) : from
      const next = players.current[to]
      if (!next) return
      levels.current[to] = 0
      applyVolume(to)
      videos.current[to] = videoId
      next.loadVideoById(videoId)
      nearEndFired.current = false
      setProgress({ current: 0, duration: 0 })
      setActiveSlot(to)
      setPlayingState(true)
      const targets: [number, number] = to === 0 ? [1, 0] : [0, 1]
      fade(targets, ms, () => {
        if (to !== from) players.current[from]?.stopVideo()
      })
    },

    // seekTo jumps within the active song.
    seekTo(seconds: number) {
      const p = players.current[activeRef.current]
      if (!p) return
      p.seekTo(seconds, true)
      setProgress((prev) => ({ ...prev, current: seconds }))
      // Seeking back out of the last few seconds re-arms the fade into the next song.
      if (progress.duration - seconds > fadeSeconds) nearEndFired.current = false
    },

    pause() {
      setPlayingState(false)
      fade([0, 0], 500, () => {
        players.current[0]?.pauseVideo()
        players.current[1]?.pauseVideo()
      })
    },

    // resume plays the active player's video (paused or only cued) and fades it in.
    resume() {
      const slot = activeRef.current
      players.current[slot]?.playVideo()
      setPlayingState(true)
      fade(slot === 0 ? [1, 0] : [0, 1], 800)
    },
  }
}
