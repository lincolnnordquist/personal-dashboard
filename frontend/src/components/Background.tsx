import { useEffect, useRef, useState, useSyncExternalStore } from 'react'

// Files in public/zelda-backgrounds/: <name>.mp4 plus posters/<name>.jpg.
const SCENES = [
  'clock-town',
  'hyrule-field',
  'kakariko',
  'laundry-pool',
  'temple-of-time',
  'tp-fountain',
  'zoras-domain',
]

// Must match the opacity transition on .background-media in index.css.
const FADE_SECONDS = 2.5

const videoUrl = (i: number) => `/zelda-backgrounds/${SCENES[i]}.mp4`
const posterUrl = (i: number) => `/zelda-backgrounds/posters/${SCENES[i]}.jpg`

/**
 * Full-screen ambient video behind the dashboard. Two <video> elements take turns: while
 * one plays, the other preloads the next scene, then they crossfade near the end.
 */
export default function Background() {
  const reducedMotion = usePrefersReducedMotion()
  const [first] = useState(() => Math.floor(Math.random() * SCENES.length))

  if (reducedMotion) {
    return (
      <div className="background" aria-hidden="true">
        <img className="background-media visible" src={posterUrl(first)} alt="" />
      </div>
    )
  }
  return <Crossfade first={first} />
}

function Crossfade({ first }: { first: number }) {
  const videos = useRef<[HTMLVideoElement | null, HTMLVideoElement | null]>([null, null])
  // Which scene each of the two video slots holds, and which slot is showing.
  const [scenes, setScenes] = useState<[number, number]>([first, (first + 1) % SCENES.length])
  const [active, setActive] = useState<0 | 1>(0)
  const fading = useRef(false)

  const play = (v: HTMLVideoElement | null) => {
    // Muted autoplay is allowed everywhere; ignore the rare rejection (e.g. power saving).
    v?.play().catch(() => {})
  }

  // Start the first scene, and pause while the tab is hidden to save CPU and battery.
  useEffect(() => {
    play(videos.current[active])
    const onVisibility = () => {
      const v = videos.current[active]
      if (document.hidden) v?.pause()
      else play(v)
    }
    document.addEventListener('visibilitychange', onVisibility)
    return () => document.removeEventListener('visibilitychange', onVisibility)
  }, [active])

  const startFade = (slot: 0 | 1) => {
    if (slot !== active || fading.current || SCENES.length < 2) return
    fading.current = true
    const next = videos.current[1 - slot]
    if (next) next.currentTime = 0
    play(next)
    setActive(slot === 0 ? 1 : 0)
  }

  const onTimeUpdate = (slot: 0 | 1) => {
    const v = videos.current[slot]
    if (v && v.duration && v.duration - v.currentTime <= FADE_SECONDS) startFade(slot)
  }

  // When a slot finishes fading out, stop it and load the scene after the one now playing.
  const onFadedOut = (slot: 0 | 1) => {
    if (slot === active || !fading.current) return
    videos.current[slot]?.pause()
    setScenes((s) => {
      const updated: [number, number] = [...s]
      updated[slot] = (s[1 - slot] + 1) % SCENES.length
      return updated
    })
    fading.current = false
  }

  return (
    <div className="background" aria-hidden="true">
      {([0, 1] as const).map((slot) => (
        <video
          key={slot}
          ref={(el) => {
            videos.current[slot] = el
          }}
          className={slot === active ? 'background-media visible' : 'background-media'}
          src={videoUrl(scenes[slot])}
          poster={posterUrl(scenes[slot])}
          muted
          playsInline
          preload="auto"
          loop={SCENES.length === 1}
          disablePictureInPicture
          onTimeUpdate={() => onTimeUpdate(slot)}
          // Fallback in case timeupdate never landed inside the fade window.
          onEnded={() => startFade(slot)}
          onTransitionEnd={(e) => e.propertyName === 'opacity' && onFadedOut(slot)}
        />
      ))}
    </div>
  )
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
