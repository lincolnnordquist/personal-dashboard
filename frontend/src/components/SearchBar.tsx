export default function SearchBar() {
  return (
    <form className="search-bar" action="https://www.google.com/search" method="get">
      <svg className="search-icon" viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
        <path
          fill="currentColor"
          d="M15.5 14h-.79l-.28-.27a6.5 6.5 0 1 0-.7.7l.27.28v.79l5 4.99L20.49 19zm-6 0A4.5 4.5 0 1 1 14 9.5 4.5 4.5 0 0 1 9.5 14"
        />
      </svg>
      <input
        type="text"
        name="q"
        placeholder="Search Google or type a URL"
        autoFocus
        autoComplete="off"
        aria-label="Search Google"
      />
    </form>
  )
}
