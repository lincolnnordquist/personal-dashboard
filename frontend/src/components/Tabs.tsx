export interface Tab<T extends string> {
  id: T
  label: string
}

// "pill" tabs sit inside a card; "label" tabs replace a widget's section label, Glance-style.
export default function Tabs<T extends string>({
  tabs,
  active,
  onChange,
  variant = 'pill',
}: {
  tabs: Tab<T>[]
  active: T
  onChange: (id: T) => void
  variant?: 'pill' | 'label'
}) {
  return (
    <div className={`tabs tabs-${variant}`} role="tablist">
      {tabs.map((t) => (
        <button
          key={t.id}
          role="tab"
          aria-selected={t.id === active}
          className={t.id === active ? 'tab active' : 'tab'}
          onClick={() => onChange(t.id)}
        >
          {t.label}
        </button>
      ))}
    </div>
  )
}
