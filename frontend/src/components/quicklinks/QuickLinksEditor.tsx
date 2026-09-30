import { useEffect, useState, type FormEvent } from 'react'
import { createPortal } from 'react-dom'
import { useMutation } from '@apollo/client/react'
import {
  GET_QUICK_LINKS,
  SAVE_QUICK_LINKS,
  type QuickLink,
  type QuickLinkGroup,
  type QuickLinks,
} from '../../graphql/queries'
import { CloseIcon, DownIcon, EditIcon, UpIcon } from '../icons'
import LinkIcon from './LinkIcon'

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

// move returns list with the item at index i moved by delta places.
function move<T>(list: T[], i: number, delta: number): T[] {
  const j = i + delta
  if (j < 0 || j >= list.length) return list
  const next = [...list]
  ;[next[i], next[j]] = [next[j], next[i]]
  return next
}

type Save = (groups: QuickLinkGroup[]) => Promise<boolean>

// QuickLinksEditor is a centered window, like the music library: groups on the left, the
// selected group's links on the right. Every change saves all the links at once, which
// rewrites links/links.txt. A save made from links that changed in the meantime (in another
// tab, or by a git pull) is refused by the server, and the latest links are loaded instead.
export default function QuickLinksEditor({
  links,
  initialGroup,
  onSaveFailed,
  onClose,
}: {
  links: QuickLinks
  initialGroup: string | null
  onSaveFailed: () => void
  onClose: () => void
}) {
  const { groups, version } = links
  const [selectedName, setSelectedName] = useState(initialGroup)
  const selectedIndex = Math.max(
    0,
    groups.findIndex((g) => g.name === selectedName),
  )
  const selected = groups[selectedIndex] ?? null
  const [error, setError] = useState<string | null>(null)

  const [saveLinks, { loading: saving }] = useMutation(SAVE_QUICK_LINKS, {
    update: (cache, { data }) => {
      if (data) cache.writeQuery({ query: GET_QUICK_LINKS, data: { quickLinks: data.saveQuickLinks } })
    },
  })

  const save: Save = async (next) => {
    try {
      await saveLinks({
        variables: {
          version,
          groups: next.map((g) => ({
            name: g.name,
            links: g.links.map((l) => ({ title: l.title, url: l.url, icon: l.icon })),
          })),
        },
      })
      setError(null)
      return true
    } catch (err) {
      setError(errorMessage(err))
      onSaveFailed()
      return false
    }
  }

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  const replaceGroup = (group: QuickLinkGroup) => groups.map((g, i) => (i === selectedIndex ? group : g))

  return createPortal(
    <div className="library-backdrop" onClick={onClose}>
      <div
        className="library"
        role="dialog"
        aria-modal="true"
        aria-label="Edit quick links"
        onClick={(e) => e.stopPropagation()}
      >
        <header className="library-header">
          <h2>Quick links</h2>
          <button className="icon-button" onClick={onClose} aria-label="Close">
            <CloseIcon />
          </button>
        </header>
        <div className="library-body">
          <GroupSidebar
            groups={groups}
            selected={selected?.name ?? null}
            onSelect={setSelectedName}
            onCreate={async (name) => {
              const ok = await save([...groups, { name, links: [] }])
              if (ok) setSelectedName(name)
              return ok
            }}
          />
          {selected ? (
            <GroupPanel
              key={selected.name}
              group={selected}
              groups={groups}
              index={selectedIndex}
              saving={saving}
              error={error}
              onRename={async (name) => {
                const ok = await save(replaceGroup({ ...selected, name }))
                if (ok) setSelectedName(name)
                return ok
              }}
              onMove={(delta) => save(move(groups, selectedIndex, delta))}
              onDelete={() => save(groups.filter((_, i) => i !== selectedIndex))}
              onLinksChange={(links) => save(replaceGroup({ ...selected, links }))}
              onEditLink={(linkIndex, link, toGroup) =>
                save(
                  groups.map((g, i) => {
                    if (toGroup === selected.name) {
                      return i === selectedIndex ? { ...g, links: g.links.map((l, j) => (j === linkIndex ? link : l)) } : g
                    }
                    if (i === selectedIndex) return { ...g, links: g.links.filter((_, j) => j !== linkIndex) }
                    return g.name === toGroup ? { ...g, links: [...g.links, link] } : g
                  }),
                )
              }
            />
          ) : (
            <div className="library-main">
              <p className="muted">Create a group to start adding links, e.g. “dev” or “games”.</p>
              {error && <p className="error small">{error}</p>}
            </div>
          )}
        </div>
      </div>
    </div>,
    document.body,
  )
}

function GroupSidebar({
  groups,
  selected,
  onSelect,
  onCreate,
}: {
  groups: QuickLinkGroup[]
  selected: string | null
  onSelect: (name: string) => void
  onCreate: (name: string) => Promise<boolean>
}) {
  const [creating, setCreating] = useState(false)
  const [name, setName] = useState('')

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return
    if (await onCreate(name.trim())) {
      setName('')
      setCreating(false)
    }
  }

  return (
    <nav className="library-sidebar" aria-label="Link groups">
      <h3 className="section-label">Groups</h3>
      <ul>
        {groups.map((g) => (
          <li key={g.name}>
            <button className={g.name === selected ? 'playlist-link active' : 'playlist-link'} onClick={() => onSelect(g.name)}>
              <span className="playlist-link-name">{g.name}</span>
              <span className="muted">{g.links.length}</span>
            </button>
          </li>
        ))}
      </ul>
      {creating ? (
        <form className="inline-form" onSubmit={onSubmit}>
          <input
            autoFocus
            value={name}
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => e.key === 'Escape' && (e.stopPropagation(), setCreating(false))}
            placeholder="Group name"
            aria-label="New group name"
          />
          <button type="submit" className="button" disabled={!name.trim()}>
            Create
          </button>
        </form>
      ) : (
        <button className="button button-quiet new-playlist" onClick={() => setCreating(true)}>
          + New group
        </button>
      )}
    </nav>
  )
}

function GroupPanel({
  group,
  groups,
  index,
  saving,
  error,
  onRename,
  onMove,
  onDelete,
  onLinksChange,
  onEditLink,
}: {
  group: QuickLinkGroup
  groups: QuickLinkGroup[]
  index: number
  saving: boolean
  error: string | null
  onRename: (name: string) => Promise<boolean>
  onMove: (delta: number) => Promise<boolean>
  onDelete: () => Promise<boolean>
  onLinksChange: (links: QuickLink[]) => Promise<boolean>
  // Saves an edited link, moving it to the end of toGroup if that's another group.
  onEditLink: (linkIndex: number, link: QuickLink, toGroup: string) => Promise<boolean>
}) {
  const [mode, setMode] = useState<'view' | 'rename' | 'confirm-delete'>('view')
  const [name, setName] = useState(group.name)
  const [editingLink, setEditingLink] = useState<number | null>(null)
  const count = group.links.length

  const onRenameSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (await onRename(name.trim())) setMode('view')
  }

  return (
    <section className="library-main">
      <div className="playlist-header">
        {mode === 'rename' ? (
          <form className="inline-form" onSubmit={onRenameSubmit}>
            <input
              autoFocus
              value={name}
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => e.key === 'Escape' && (e.stopPropagation(), setMode('view'))}
              aria-label="Group name"
            />
            <button type="submit" className="button" disabled={!name.trim()}>
              Save
            </button>
            <button type="button" className="button button-quiet" onClick={() => setMode('view')}>
              Cancel
            </button>
          </form>
        ) : mode === 'confirm-delete' ? (
          <div className="confirm">
            <span>
              Delete “{group.name}” and its {count} {count === 1 ? 'link' : 'links'}?
            </span>
            <button className="button button-danger" onClick={onDelete}>
              Delete
            </button>
            <button className="button button-quiet" onClick={() => setMode('view')}>
              Cancel
            </button>
          </div>
        ) : (
          <>
            <div>
              <h3 className="playlist-title">{group.name}</h3>
              <span className="muted">
                {count} {count === 1 ? 'link' : 'links'}
              </span>
            </div>
            <div className="playlist-actions">
              <button
                className="icon-button"
                onClick={() => onMove(-1)}
                disabled={index === 0}
                aria-label="Move group earlier"
                title="Move group earlier"
              >
                <UpIcon />
              </button>
              <button
                className="icon-button"
                onClick={() => onMove(1)}
                disabled={index === groups.length - 1}
                aria-label="Move group later"
                title="Move group later"
              >
                <DownIcon />
              </button>
              <button className="button button-quiet" onClick={() => (setName(group.name), setMode('rename'))}>
                Rename
              </button>
              <button className="button button-quiet" onClick={() => setMode('confirm-delete')}>
                Delete
              </button>
            </div>
          </>
        )}
      </div>

      <LinkForm
        submitLabel="Add"
        saving={saving}
        onSubmit={(link) => onLinksChange([...group.links, link])}
      />
      {error && <p className="error small">{error}</p>}

      <ul className="link-list">
        {count === 0 && <li className="muted">No links in this group yet. Add one above.</li>}
        {group.links.map((link, i) =>
          editingLink === i ? (
            <li key={i} className="link-row editing">
              <LinkForm
                initial={link}
                submitLabel="Save"
                saving={saving}
                groups={groups.map((g) => g.name)}
                group={group.name}
                onCancel={() => setEditingLink(null)}
                onSubmit={async (edited, toGroup) => {
                  const ok = await onEditLink(i, edited, toGroup ?? group.name)
                  if (ok) setEditingLink(null)
                  return ok
                }}
              />
            </li>
          ) : (
            <li key={i} className="link-row">
              <LinkIcon link={link} />
              <span className="link-row-text">
                <span className="link-row-title">{link.title}</span>
                <span className="link-row-url muted">{link.url}</span>
              </span>
              <button
                className="icon-button"
                onClick={() => onLinksChange(move(group.links, i, -1))}
                disabled={i === 0}
                aria-label={`Move “${link.title}” earlier`}
                title="Move earlier"
              >
                <UpIcon />
              </button>
              <button
                className="icon-button"
                onClick={() => onLinksChange(move(group.links, i, 1))}
                disabled={i === count - 1}
                aria-label={`Move “${link.title}” later`}
                title="Move later"
              >
                <DownIcon />
              </button>
              <button
                className="icon-button"
                onClick={() => setEditingLink(i)}
                aria-label={`Edit “${link.title}”`}
                title="Edit"
              >
                <EditIcon />
              </button>
              <button
                className="icon-button"
                onClick={() => onLinksChange(group.links.filter((_, j) => j !== i))}
                aria-label={`Remove “${link.title}”`}
                title="Remove"
              >
                <CloseIcon />
              </button>
            </li>
          ),
        )}
      </ul>
    </section>
  )
}

// LinkForm adds or edits a link. When editing, it can also move the link to another group.
function LinkForm({
  initial,
  submitLabel,
  saving,
  groups,
  group,
  onSubmit,
  onCancel,
}: {
  initial?: QuickLink
  submitLabel: string
  saving: boolean
  groups?: string[]
  group?: string
  onSubmit: (link: QuickLink, toGroup?: string) => Promise<boolean>
  onCancel?: () => void
}) {
  const [url, setUrl] = useState(initial?.url ?? '')
  const [title, setTitle] = useState(initial?.title ?? '')
  const [icon, setIcon] = useState(initial?.icon ?? '')
  const [toGroup, setToGroup] = useState(group)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (!url.trim()) return
    const ok = await onSubmit({ url: url.trim(), title: title.trim(), icon: icon.trim(), faviconUrl: '' }, toGroup)
    if (ok && !initial) {
      setUrl('')
      setTitle('')
      setIcon('')
    }
  }

  return (
    <form
      className="link-form"
      onSubmit={submit}
      onKeyDown={(e) => e.key === 'Escape' && onCancel && (e.stopPropagation(), onCancel())}
    >
      <input
        autoFocus={!!initial}
        value={url}
        onChange={(e) => setUrl(e.target.value)}
        placeholder="Link, e.g. github.com"
        aria-label="Link URL"
      />
      <input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Name (optional)" aria-label="Name" />
      <input
        className="link-form-icon"
        value={icon}
        onChange={(e) => setIcon(e.target.value)}
        placeholder="Icon (emoji or image URL)"
        aria-label="Icon override: an emoji or an image URL"
        title="Leave empty to use the site's own icon"
      />
      {groups && groups.length > 1 && (
        <select value={toGroup} onChange={(e) => setToGroup(e.target.value)} aria-label="Group">
          {groups.map((g) => (
            <option key={g} value={g}>
              {g}
            </option>
          ))}
        </select>
      )}
      <div className="link-form-buttons">
        <button type="submit" className="button" disabled={saving || !url.trim()}>
          {submitLabel}
        </button>
        {onCancel && (
          <button type="button" className="button button-quiet" onClick={onCancel}>
            Cancel
          </button>
        )}
      </div>
    </form>
  )
}
