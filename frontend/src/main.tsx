import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { ApolloClient, HttpLink, InMemoryCache } from '@apollo/client'
import { ApolloProvider } from '@apollo/client/react'
import './index.css'
import App from './App.tsx'

const client = new ApolloClient({
  // Same origin as the page: nginx (or the Vite dev server) forwards /graphql to the backend.
  link: new HttpLink({ uri: '/graphql' }),
  cache: new InMemoryCache({
    typePolicies: {
      // A game's home/away objects have no id, so tell Apollo it is safe to merge them
      // when the same game arrives from both the scoreboard and a team schedule.
      Game: { fields: { home: { merge: true }, away: { merge: true } } },
    },
  }),
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ApolloProvider client={client}>
      <App />
    </ApolloProvider>
  </StrictMode>,
)
