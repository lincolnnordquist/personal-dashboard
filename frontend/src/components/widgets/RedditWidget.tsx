import { useState } from 'react'
import { useQuery } from '@apollo/client/react'
import { GET_REDDIT, type RedditPost } from '../../graphql/queries'
import { compactNumber, timeAgo } from '../../format'
import type { WidgetProps } from '../Grid'
import Tabs from '../Tabs'
import WidgetCard from '../WidgetCard'

const REFRESH_MS = 5 * 60 * 1000

export default function RedditWidget({ config }: WidgetProps) {
  const subreddits = Array.isArray(config.subreddits)
    ? config.subreddits.filter((s): s is string => typeof s === 'string')
    : []

  // errorPolicy 'all' keeps the feeds that loaded when one subreddit fails.
  const { data, loading, error } = useQuery(GET_REDDIT, {
    variables: { subreddits },
    skip: subreddits.length === 0,
    pollInterval: REFRESH_MS,
    errorPolicy: 'all',
  })
  const feeds = data?.redditData ?? []
  const [active, setActive] = useState<string | null>(null)
  const feed = feeds.find((f) => f.subreddit === active) ?? feeds[0]

  return (
    <WidgetCard title="Reddit">
      {subreddits.length === 0 && <p className="muted">No subreddits configured.</p>}
      {loading && feeds.length === 0 && <p className="muted">Loading…</p>}
      {error && <p className="error">{error.message}</p>}

      {feeds.length > 1 && feed && (
        <Tabs
          tabs={feeds.map((f) => ({ id: f.subreddit, label: `r/${f.subreddit}` }))}
          active={feed.subreddit}
          onChange={setActive}
        />
      )}

      {feed && (
        <ul className="post-list tab-panel">
          {feed.posts.length === 0 && <li className="muted">No posts today.</li>}
          {feed.posts.map((p) => (
            <Post key={p.url} post={p} />
          ))}
        </ul>
      )}
    </WidgetCard>
  )
}

function Post({ post }: { post: RedditPost }) {
  const meta = [
    post.score !== null && `${compactNumber(post.score)} pts`,
    post.commentCount !== null && `${compactNumber(post.commentCount)} comments`,
    timeAgo(post.publishedAt),
    `u/${post.author}`,
  ].filter(Boolean)

  return (
    <li className="post">
      <a href={post.url} target="_blank" rel="noreferrer">
        {post.title}
      </a>
      <div className="post-meta muted">{meta.join(' · ')}</div>
    </li>
  )
}
