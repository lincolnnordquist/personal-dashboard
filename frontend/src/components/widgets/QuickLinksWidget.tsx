import { useState } from 'react'
import { useQuery } from '@apollo/client/react'
import { GET_QUICK_LINKS } from '../../graphql/queries'
import { useStoredState } from '../../lib/storage'
import { EditIcon } from '../icons'
import LinkIcon from '../quicklinks/LinkIcon'
import QuickLinksEditor from '../quicklinks/QuickLinksEditor'
import Tabs from '../Tabs'
import WidgetCard from '../WidgetCard'

// The links file can change on the other machine (through git), so check now and then.
const REFRESH_MS = 60 * 1000

// QuickLinksWidget is a speed dial: a grid of site icons, one tab per group. The links live in
// links/links.txt in the repo and are edited in a window opened with the pencil button.
export default function QuickLinksWidget() {
  const { data, loading, error, refetch } = useQuery(GET_QUICK_LINKS, { pollInterval: REFRESH_MS })
  const groups = data?.quickLinks.groups ?? []
  const [activeName, setActiveName] = useStoredState<string | null>('quicklinks-group', null)
  const group = groups.find((g) => g.name === activeName) ?? groups[0]
  const [editing, setEditing] = useState(false)

  const editButton = (
    <button
      className="icon-button heading-button"
      onClick={() => setEditing(true)}
      aria-label="Edit quick links"
      title="Edit quick links"
    >
      <EditIcon />
    </button>
  )

  return (
    <WidgetCard
      header={
        <div className="heading-with-action">
          {groups.length > 1 && group ? (
            <Tabs
              tabs={groups.map((g) => ({ id: g.name, label: g.name }))}
              active={group.name}
              onChange={setActiveName}
              variant="label"
            />
          ) : (
            <h2 className="widget-title">{group?.name ?? 'Quick links'}</h2>
          )}
          {editButton}
        </div>
      }
    >
      {editing && data && (
        <QuickLinksEditor
          links={data.quickLinks}
          initialGroup={group?.name ?? null}
          onSaveFailed={() => refetch()}
          onClose={() => setEditing(false)}
        />
      )}
      {loading && !data && <p className="muted">Loading…</p>}
      {error && <p className="error">{error.message}</p>}
      {data && !group?.links.length && (
        <button className="button button-quiet" onClick={() => setEditing(true)}>
          Add links
        </button>
      )}

      {group && group.links.length > 0 && (
        <ul className="quicklinks">
          {group.links.map((link, i) => (
            <li key={`${i}-${link.url}`}>
              <a className="quicklink" href={link.url} target="_blank" rel="noreferrer" title={link.url}>
                <LinkIcon link={link} />
                <span className="quicklink-title">{link.title}</span>
              </a>
            </li>
          ))}
        </ul>
      )}
    </WidgetCard>
  )
}
