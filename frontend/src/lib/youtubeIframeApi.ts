let loading: Promise<typeof YT> | null = null

// loadYouTubeIframeApi loads YouTube's IFrame player API once and resolves with the YT global.
export function loadYouTubeIframeApi(): Promise<typeof YT> {
  loading ??= new Promise((resolve) => {
    if (window.YT?.Player) {
      resolve(window.YT)
      return
    }
    const previous = window.onYouTubeIframeAPIReady
    window.onYouTubeIframeAPIReady = () => {
      previous?.()
      resolve(window.YT)
    }
    const script = document.createElement('script')
    script.src = 'https://www.youtube.com/iframe_api'
    document.head.appendChild(script)
  })
  return loading
}
