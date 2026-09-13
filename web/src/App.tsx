import { useEffect, useState } from 'react'
import { AboutPage } from './pages/About.tsx'
import { CreatePage } from './pages/Create.tsx'
import { PrivacyPage } from './pages/Privacy.tsx'
import { TermsPage } from './pages/Terms.tsx'

function selectedCode(pathname: string): string | null {
  const match = pathname.match(/^\/app\/links\/([^/]+)\/?$/)
  if (match) {
    return decodeURIComponent(match[1])
  }
  return null
}

export function App() {
  const [path, setPath] = useState(() => window.location.pathname)

  useEffect(() => {
    const onPop = () => setPath(window.location.pathname)
    window.addEventListener('popstate', onPop)
    return () => window.removeEventListener('popstate', onPop)
  }, [])

  function go(to: string) {
    window.history.pushState(null, '', to)
    setPath(to)
  }

  if (path === '/app/about' || path === '/app/about/') {
    return <AboutPage />
  }
  if (path === '/app/terms' || path === '/app/terms/') {
    return <TermsPage />
  }
  if (path === '/app/privacy' || path === '/app/privacy/') {
    return <PrivacyPage />
  }

  return (
    <CreatePage
      selected={selectedCode(path)}
      onOpen={(code) => go(`/app/links/${encodeURIComponent(code)}`)}
      onClear={() => go('/app/')}
    />
  )
}
