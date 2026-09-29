import { useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react'
import { shortcutsBlocked } from '../lib/keyboard'
import { useStoredState } from '../lib/storage'

export interface Scene {
  url: string
  posterUrl: string | null
}

type Slot = 0 | 1

// Must match the opacity transition on .background-frame in index.css.
const FADE_SECONDS = 2.5

// cover fills the screen, cropping and scaling; native shows videos at their real pixel size,
// centered with faded edges (scaled down only if bigger than the window).
type Fit = 'cover' | 'native'

/**
 * Full-screen ambient video behind the dashboard, cycling through `scenes` in a shuffled order.
 * Two <video> elements take turns: while one plays, the other preloads the next scene, then
 * they crossfade near the end. When `scenes` changes (a new background theme), it crossfades
 * to the new theme right away. Pass null while the theme is still loading.
 *
 * Keyboard: B skips to the next scene of the current theme, Shift+B goes back one, and V
 * switches between filling the screen and showing videos at their actual size.
 */
export default function Background({ scenes }: { scenes: Scene[] | null }) {
  const reducedMotion = usePrefersReducedMotion()
  // One shuffle seed per page load, so each theme plays in a varied but stable order.
  const [seed] = useState(Math.random)
  const order = useMemo(() => (scenes ? shuffled(scenes, seed) : []), [scenes, seed])

  const [fit, setFit] = useStoredState<Fit>('background-fit', 'cover')
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key.toLowerCase() !== 'v' || shortcutsBlocked(e)) return
      setFit(fit === 'cover' ? 'native' : 'cover')
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [fit, setFit])

  const className = `background fit-${fit}`
  if (order.length === 0) return <div className={className} aria-hidden="true" />
  if (reducedMotion) {
    const still = order.find((s) => s.posterUrl)
    return (
      <div className={className} aria-hidden="true">
        {still && (
          <div className="background-frame visible">
            <img className="background-media" src={still.posterUrl!} alt="" />
          </div>
        )}
      </div>
    )
  }
  return <Crossfade order={order} className={className} />
}

interface CrossfadeState {
  order: Scene[]
  // The scene each video element holds, and which one is showing.
  slots: [Scene, Scene | null]
  active: Slot
  // The showing scene's index in order.
  pos: number
  // True after a manual skip, which uses a much shorter crossfade (.quick-fade in index.css).
  quick: boolean
}

function initialState(order: Scene[]): CrossfadeState {
  return { order, slots: [order[0], order.length > 1 ? order[1] : null], active: 0, pos: 0, quick: false }
}

function Crossfade({ order, className }: { order: Scene[]; className: string }) {
  const videos = useRef<[HTMLVideoElement | null, HTMLVideoElement | null]>([null, null])
  const fading = useRef(false)
  const [state, setState] = useState(() => initialState(order))

  // A new theme: load its first scene into the hidden player and fade over to it now.
  if (state.order !== order) {
    const hidden: Slot = state.active === 0 ? 1 : 0
    const slots: CrossfadeState['slots'] = [...state.slots]
    slots[hidden] = order[0]
    setState({ order, slots, active: hidden, pos: 0, quick: false })
  }

  const { slots, active } = state
  const activeUrl = slots[active]?.url

  const play = (v: HTMLVideoElement | null) => {
    // Muted autoplay is allowed everywhere; ignore the rare rejection (e.g. power saving).
    v?.play().catch(() => {})
  }

  // Play whichever scene is showing, and pause while the tab is hidden to save CPU and battery.
  useEffect(() => {
    play(videos.current[active])
    const onVisibility = () => {
      const v = videos.current[active]
      if (document.hidden) v?.pause()
      else play(v)
    }
    document.addEventListener('visibilitychange', onVisibility)
    return () => document.removeEventListener('visibilitychange', onVisibility)
  }, [active, activeUrl])

  const startFade = (slot: Slot) => {
    if (slot !== active || fading.current || state.order.length < 2) return
    fading.current = true
    const next = videos.current[slot === 0 ? 1 : 0]
    if (next) next.currentTime = 0
    play(next)
    setState((s) => ({ ...s, active: slot === 0 ? 1 : 0, pos: (s.pos + 1) % s.order.length, quick: false }))
  }

  // skip jumps to the next or previous scene with a quick crossfade. It loads that scene into
  // the hidden player (the next one is usually preloaded there already) and swaps. Presses
  // mid-fade work too, so holding or tapping B flicks through scenes.
  const skip = (direction: 1 | -1) => {
    const n = state.order.length
    if (n < 2) return
    fading.current = true
    const hidden: Slot = active === 0 ? 1 : 0
    const pos = (state.pos + direction + n) % n
    const scene = state.order[pos]
    // A player that already holds this scene may be partway through it: start from the top.
    const player = videos.current[hidden]
    if (player && slots[hidden] === scene) player.currentTime = 0
    setState((s) => {
      const slots: CrossfadeState['slots'] = [...s.slots]
      slots[hidden] = scene
      return { ...s, slots, active: hidden, pos, quick: true }
    })
  }

  // The key listener is registered once and calls the latest skip through a ref.
  const skipRef = useRef(skip)
  useEffect(() => {
    skipRef.current = skip
  })
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key.toLowerCase() !== 'b' || shortcutsBlocked(e)) return
      skipRef.current(e.shiftKey ? -1 : 1)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  const onTimeUpdate = (slot: Slot) => {
    const v = videos.current[slot]
    if (v && v.duration && v.duration - v.currentTime <= FADE_SECONDS) startFade(slot)
  }

  // When a player finishes fading out, stop it and preload the scene after the one showing.
  const onFadedOut = (slot: Slot) => {
    if (slot === active) return
    videos.current[slot]?.pause()
    setState((s) => {
      const slots: CrossfadeState['slots'] = [...s.slots]
      slots[slot] = s.order[(s.pos + 1) % s.order.length]
      return { ...s, slots }
    })
    fading.current = false
  }

  return (
    <div className={state.quick ? `${className} quick-fade` : className} aria-hidden="true">
      {([0, 1] as const).map((slot) => {
        const scene = slots[slot]
        // The frame fades in and out, and in actual-size mode wraps the video tightly so its
        // edge fade (a static overlay, far cheaper than a mask on the video) lines up with it.
        return (
          <div
            key={slot}
            className={slot === active ? 'background-frame visible' : 'background-frame'}
            onTransitionEnd={(e) => e.target === e.currentTarget && e.propertyName === 'opacity' && onFadedOut(slot)}
          >
            <video
              ref={(el) => {
                videos.current[slot] = el
              }}
              className="background-media"
              src={scene?.url}
              poster={scene?.posterUrl ?? undefined}
              muted
              playsInline
              preload="auto"
              loop={state.order.length === 1}
              disablePictureInPicture
              onTimeUpdate={() => onTimeUpdate(slot)}
              // Fallback in case timeupdate never landed inside the fade window.
              onEnded={() => startFade(slot)}
            />
          </div>
        )
      })}
    </div>
  )
}

// shuffled orders scenes by a hash of each URL and the seed: a shuffle that stays the same for
// the same inputs, so rendering is pure and switching back to a theme resumes the same order.
function shuffled(scenes: Scene[], seed: number): Scene[] {
  const salt = Math.floor(seed * 0xffffffff)
  const hash = (s: string) => {
    let h = salt
    for (let i = 0; i < s.length; i++) h = Math.imul(h ^ s.charCodeAt(i), 0x9e3779b1) >>> 0
    return h
  }
  return [...scenes].sort((a, b) => hash(a.url) - hash(b.url))
}

const reducedMotionQuery = '(prefers-reduced-motion: reduce)'

function usePrefersReducedMotion(): boolean {
  return useSyncExternalStore(
    (onChange) => {
      const mq = window.matchMedia(reducedMotionQuery)
      mq.addEventListener('change', onChange)
      return () => mq.removeEventListener('change', onChange)
    },
    () => window.matchMedia(reducedMotionQuery).matches,
  )
}
