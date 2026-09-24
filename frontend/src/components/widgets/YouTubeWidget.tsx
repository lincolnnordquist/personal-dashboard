import { useQuery } from '@apollo/client/react'
import { GET_YOUTUBE, type YoutubeVideo } from '../../graphql/queries'
import { compactNumber, timeAgo, videoLength } from '../../format'
import type { WidgetProps } from '../Grid'
import WidgetCard from '../WidgetCard'

const REFRESH_MS = 10 * 60 * 1000
const MAX_VIDEOS = 24

export default function YouTubeWidget({ config }: WidgetProps) {
  const channelIds = Array.isArray(config.channelIds)
    ? config.channelIds.filter((c): c is string => typeof c === 'string')
    : []

  // errorPolicy 'all' keeps the channels that loaded when one fails.
  const { data, loading, error } = useQuery(GET_YOUTUBE, {
    variables: { channelIds },
    skip: channelIds.length === 0,
    pollInterval: REFRESH_MS,
    errorPolicy: 'all',
  })

  // One row of every channel's uploads, newest first.
  const videos = (data?.youtubeData ?? [])
    .flatMap((ch) => ch.videos.map((video) => ({ video, channel: ch.channelName })))
    .sort((a, b) => b.video.publishedAt.localeCompare(a.video.publishedAt))
    .slice(0, MAX_VIDEOS)

  return (
    <WidgetCard title="Videos">
      {channelIds.length === 0 && <p className="muted">No channels configured.</p>}
      {loading && videos.length === 0 && <p className="muted">Loading…</p>}
      {error && <p className="error">{error.message}</p>}
      {videos.length > 0 && (
        <ul className="video-row">
          {videos.map(({ video, channel }) => (
            <VideoCard key={video.videoId} video={video} channel={channel} />
          ))}
        </ul>
      )}
    </WidgetCard>
  )
}

function VideoCard({ video, channel }: { video: YoutubeVideo; channel: string }) {
  const meta = [
    timeAgo(video.publishedAt),
    video.viewCount !== null && `${compactNumber(video.viewCount)} views`,
  ].filter(Boolean)

  return (
    <li className="video-card">
      <a href={`https://www.youtube.com/watch?v=${video.videoId}`} target="_blank" rel="noreferrer">
        <div className="video-thumb">
          {video.thumbnailUrl && <img src={video.thumbnailUrl} alt="" loading="lazy" />}
          <span className="video-length">{videoLength(video.durationSeconds)}</span>
        </div>
        <div className="video-title" title={video.title}>
          {video.title}
        </div>
      </a>
      <div className="video-channel">{channel}</div>
      <div className="video-meta muted">{meta.join(' · ')}</div>
    </li>
  )
}
