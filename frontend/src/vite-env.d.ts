/// <reference types="vite/client" />

interface Window {
  // Called by YouTube's IFrame API script once it has loaded.
  onYouTubeIframeAPIReady?: () => void
}
