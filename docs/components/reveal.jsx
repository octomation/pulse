'use client'

// Fades a block in when it scrolls into view, and marks it .in-view so the
// motion inside it starts only when somebody can see it.
import { useEffect, useRef, useState } from 'react'

export function Reveal({ as: Tag = 'div', className = '', children }) {
  const ref = useRef(null)
  const [seen, setSeen] = useState(false)
  useEffect(() => {
    const node = ref.current
    if (!node || typeof IntersectionObserver === 'undefined') {
      setSeen(true)
      return
    }
    const observer = new IntersectionObserver(entries => {
      if (entries.some(entry => entry.isIntersecting)) {
        setSeen(true)
        observer.disconnect()
      }
    }, { rootMargin: '0px 0px -12% 0px' })
    observer.observe(node)
    return () => observer.disconnect()
  }, [])
  return <Tag ref={ref} className={`reveal${seen ? ' is-in in-view' : ''} ${className}`.trim()}>{children}</Tag>
}
