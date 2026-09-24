import { useQuery } from '@apollo/client/react'
import { GET_DOCKER, type DockerContainer } from '../../graphql/queries'
import WidgetCard from '../WidgetCard'

// Docker status is fetched live by the backend on every request, so poll often.
const REFRESH_MS = 30 * 1000

export default function DockerWidget() {
  const { data, loading, error } = useQuery(GET_DOCKER, { pollInterval: REFRESH_MS })
  const containers = data?.dockerData ?? []
  const running = containers.filter((c) => c.status === 'running').length

  // Group Compose containers by project. Projects with running containers come first,
  // then by name, with standalone containers last.
  const groups = new Map<string, DockerContainer[]>()
  for (const c of containers) {
    const key = c.project ?? ''
    groups.set(key, [...(groups.get(key) ?? []), c])
  }
  const isUp = (items: DockerContainer[]) => items.some((c) => c.status === 'running')
  const ordered = [...groups.entries()].sort(
    ([a, aItems], [b, bItems]) =>
      Number(isUp(bItems)) - Number(isUp(aItems)) || Number(a === '') - Number(b === '') || a.localeCompare(b),
  )

  return (
    <WidgetCard title="Docker">
      {error && <p className="error">{error.message}</p>}
      {loading && !data && <p className="muted">Loading…</p>}
      {data && (
        <>
          <p className="muted docker-summary">
            {running} running · {containers.length - running} stopped
          </p>
          <div className="tab-panel">
            {ordered.map(([project, items]) => (
              <section key={project}>
                <h3 className="section-label">{project || 'Standalone'}</h3>
                <ul className="containers">
                  {items.map((c) => (
                    <ContainerRow key={c.id} container={c} />
                  ))}
                </ul>
              </section>
            ))}
          </div>
        </>
      )}
    </WidgetCard>
  )
}

function ContainerRow({ container: c }: { container: DockerContainer }) {
  const tone = statusTone(c)
  return (
    <li className="container-row">
      <span className={`status-dot ${tone}`} title={c.health ? `${c.status} (${c.health})` : c.status} />
      <div className="container-main">
        <div className="container-name">{c.service ?? c.name}</div>
        <div className="container-image muted">{c.image}</div>
      </div>
      <div className={`container-state ${tone === 'ok' ? 'muted' : tone}`}>
        {c.status === 'running' ? (
          <>
            {c.health && c.health !== 'healthy' ? c.health : 'up'} {c.uptime}
          </>
        ) : (
          c.statusText
        )}
      </div>
    </li>
  )
}

function statusTone(c: DockerContainer): 'ok' | 'warn' | 'bad' | 'off' {
  if (c.status === 'running') {
    if (c.health === 'unhealthy') return 'bad'
    if (c.health === 'starting') return 'warn'
    return 'ok'
  }
  if (c.status === 'restarting' || c.status === 'paused') return 'warn'
  if (c.status === 'dead') return 'bad'
  return 'off'
}
