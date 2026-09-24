import { useQuery } from '@apollo/client/react'
import { GET_WIDGETS } from './graphql/queries'
import Background from './components/Background'
import Grid from './components/Grid'

export default function App() {
  const { data, loading, error } = useQuery(GET_WIDGETS)

  return (
    <>
      <Background />
      <main className="app">
        <header className="app-header">
          <h1>Dashboard</h1>
        </header>
        {loading && <p className="muted">Loading widgets…</p>}
        {error && <p className="error">Could not load widgets: {error.message}</p>}
        {data && <Grid widgets={data.widgets.filter((w) => w.enabled)} />}
      </main>
    </>
  )
}
