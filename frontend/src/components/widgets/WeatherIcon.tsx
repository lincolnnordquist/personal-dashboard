// Small inline weather icons, one per condition group returned by the backend's
// WeatherCondition (backend/widgets/weather.go).

function Sun() {
  return (
    <>
      <circle cx="12" cy="12" r="4.5" fill="currentColor" />
      <g stroke="currentColor" strokeWidth="1.6" strokeLinecap="round">
        <path d="M12 2.5v2.4M12 19.1v2.4M21.5 12h-2.4M4.9 12H2.5M18.5 5.5l-1.7 1.7M7.2 16.8l-1.7 1.7M18.5 18.5l-1.7-1.7M7.2 7.2 5.5 5.5" />
      </g>
    </>
  )
}

function Cloud({ muted = false }: { muted?: boolean }) {
  return (
    <path
      fill="currentColor"
      opacity={muted ? 0.6 : 1}
      d="M7.5 18A4.5 4.5 0 0 1 6.8 9.08 5.5 5.5 0 0 1 17.37 8 4 4 0 0 1 17 16H7.5z"
    />
  )
}

function CloudSun() {
  return (
    <>
      <g transform="translate(-2.5,-2.5) scale(0.62)">
        <Sun />
      </g>
      <Cloud />
    </>
  )
}

function Drops({ count }: { count: number }) {
  const xs = count === 1 ? [12] : count === 2 ? [9.5, 14.5] : [8, 12, 16]
  return (
    <g stroke="currentColor" strokeWidth="1.6" strokeLinecap="round">
      {xs.map((x) => (
        <path key={x} d={`M${x} 18v2.8`} />
      ))}
    </g>
  )
}

function Flakes({ count }: { count: number }) {
  const xs = count === 1 ? [12] : [9, 15]
  return (
    <g fill="currentColor">
      {xs.map((x) => (
        <circle key={x} cx={x} cy="19.4" r="1.1" />
      ))}
    </g>
  )
}

function Bolt() {
  return <path fill="currentColor" d="m12.5 17-2.2 4.6L13 20l-1.2 3 4.7-5.2-3.1.3z" />
}

function Fog() {
  return (
    <g stroke="currentColor" strokeWidth="1.6" strokeLinecap="round">
      <path d="M4 10h16M4 14h16M4 18h12" />
    </g>
  )
}

// condition is the free-text string from WeatherCondition, e.g. "Rain Showers".
export default function WeatherIcon({ condition, size = 22 }: { condition: string; size?: number }) {
  const body = (() => {
    switch (condition) {
      case 'Clear':
      case 'Mostly Clear':
        return <Sun />
      case 'Partly Cloudy':
        return <CloudSun />
      case 'Fog':
        return (
          <>
            <Cloud muted />
            <Fog />
          </>
        )
      case 'Drizzle':
      case 'Freezing Drizzle':
        return (
          <>
            <Cloud />
            <Drops count={1} />
          </>
        )
      case 'Rain':
      case 'Freezing Rain':
        return (
          <>
            <Cloud />
            <Drops count={3} />
          </>
        )
      case 'Rain Showers':
        return (
          <>
            <Cloud />
            <Drops count={2} />
          </>
        )
      case 'Snow':
      case 'Snow Showers':
        return (
          <>
            <Cloud />
            <Flakes count={2} />
          </>
        )
      case 'Thunderstorm':
      case 'Thunderstorm with Hail':
        return (
          <>
            <Cloud />
            <Bolt />
          </>
        )
      case 'Overcast':
      default:
        return <Cloud />
    }
  })()

  return (
    <svg viewBox="0 0 24 24" width={size} height={size} aria-hidden="true">
      {body}
    </svg>
  )
}
