import { useEffect } from 'react'

/**
 * Names the browser tab and history entry after the page, and gives the
 * home page's title back when the page is left.
 */
export function usePageTitle(title: string) {
  useEffect(() => {
    const previous = document.title
    document.title = `${title} · Cerebellum`
    return () => {
      document.title = previous
    }
  }, [title])
}
