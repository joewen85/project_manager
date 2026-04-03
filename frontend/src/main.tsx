import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from './App'
import './styles.css'

function ViewportBridge() {
  React.useEffect(() => {
    const root = document.documentElement
    const horizontalFold = window.matchMedia('(horizontal-viewport-segments: 2)')
    const verticalFold = window.matchMedia('(vertical-viewport-segments: 2)')
    const observeMediaQuery = (query: MediaQueryList, listener: () => void) => {
      if (query.addEventListener) {
        query.addEventListener('change', listener)
        return () => query.removeEventListener('change', listener)
      }
      query.addListener(listener)
      return () => query.removeListener(listener)
    }

    const syncViewport = () => {
      const viewport = window.visualViewport
      const width = viewport?.width ?? window.innerWidth
      const height = viewport?.height ?? window.innerHeight
      root.style.setProperty('--app-width', `${window.innerWidth}px`)
      root.style.setProperty('--app-height', `${window.innerHeight}px`)
      root.style.setProperty('--visual-viewport-width', `${width}px`)
      root.style.setProperty('--visual-viewport-height', `${height}px`)
      root.dataset.orientation = width >= height ? 'landscape' : 'portrait'
      root.dataset.foldLayout = horizontalFold.matches
        ? 'dual-landscape'
        : verticalFold.matches
          ? 'dual-portrait'
          : 'single'
    }

    syncViewport()
    window.addEventListener('resize', syncViewport)
    window.addEventListener('orientationchange', syncViewport)
    const stopHorizontalFold = observeMediaQuery(horizontalFold, syncViewport)
    const stopVerticalFold = observeMediaQuery(verticalFold, syncViewport)
    window.visualViewport?.addEventListener('resize', syncViewport)
    window.visualViewport?.addEventListener('scroll', syncViewport)

    return () => {
      window.removeEventListener('resize', syncViewport)
      window.removeEventListener('orientationchange', syncViewport)
      stopHorizontalFold()
      stopVerticalFold()
      window.visualViewport?.removeEventListener('resize', syncViewport)
      window.visualViewport?.removeEventListener('scroll', syncViewport)
    }
  }, [])

  return null
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <ViewportBridge />
      <App />
    </BrowserRouter>
  </React.StrictMode>
)
