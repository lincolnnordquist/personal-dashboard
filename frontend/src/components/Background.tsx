import { useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react'

export interface Scene {
  url: string
  posterUrl: string | null
}

type Slot = 0 | 1

// Must match the opacity transition on .background-media in index.css.
const FADE_SECONDS = 2.5

/**
 * Full-screen ambient video behind the dashboard, cycling through `scenes` in a shuffled order.
 * Two <video> elements take turns: while one plays, the other preloads the next scene, then
 * they crossfade near the end. When `scenes` changes (a new background theme), it crossfades
 * to the new theme right away. Pass null while the theme is still loading.
 */
export default function Background({ scenes }: { scenes: Scene[] | null }) {
  const reducedMotion = usePrefersReducedMotion()
  // One shuffle seed per page load, so each theme plays in a varied but stable order.
  const [seed] = useState(Math.random)
  const order = useMemo(() => (scenes ? shuffled(scenes, seed) : []), [scenes, seed])

  if (order.length === 0) return <div className="background" aria-hidden="true" />
  if (reducedMotion) {
    const still = order.find((s) => s.posterUrl)
    return (
      <div className="background" aria-hidden="true">
        {still && <img className="background-media visible" src={still.posterUrl!} alt="" />}
      </div>
    )
  }
  return <Crossfade order={order} />
}

interface CrossfadeState {
  order: Scene[]
  // The scene each video element holds, and which one is showing.
  slots: [Scene, Scene | null]
  active: Slot
  // The showing scene's index in order.
  pos: number
}

function initialState(order: Scene[]): CrossfadeState {
  return { order, slots: [order[0], order.length > 1 ? order[1] : null], active: 0, pos: 0 }
}

function Crossfade({ order }: { order: Scene[] }) {
  const videos = useRef<[HTMLVideoElement | null, HTMLVideoElement | null]>([null, null])
  const fading = useRef(false)
  const [state, setState] = useState(() => initialState(order))

  // A new theme: load its first scene into the hidden player and fade over to it now.
  if (state.order !== order) {
    const hidden: Slot = state.active === 0 ? 1 : 0
    const slots: CrossfadeState['slots'] = [...state.slots]
    slots[hidden] = order[0]
    setState({ order, slots, active: hidden, pos: 0 })
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
    setState((s) => ({ ...s, active: slot === 0 ? 1 : 0, pos: (s.pos + 1) % s.order.length }))
  }

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
    <div className="background" aria-hidden="true">
      {([0, 1] as const).map((slot) => {
        const scene = slots[slot]
        return (
          <video
            key={slot}
            ref={(el) => {
              videos.current[slot] = el
            }}
            className={slot === active ? 'background-media visible' : 'background-media'}
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
            onTransitionEnd={(e) => e.propertyName === 'opacity' && onFadedOut(slot)}
          />
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
