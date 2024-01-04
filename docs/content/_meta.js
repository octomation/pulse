export default {
  '*': { theme: { timestamp: false } },
  index: {
    title: 'Home', type: 'page', display: 'hidden',
    theme: { layout: 'full', sidebar: false, toc: false, breadcrumb: false, pagination: false, copyPage: false },
  },
  documentation: { title: 'Documentation', type: 'page', href: '/trend-watch/' },
  releases: { title: 'Changelog', type: 'page', href: '/changelog/' },
  'trend-watch': 'Trend watch',
  releaseHistory: { title: 'Changelog', type: 'separator' },
  changelog: { title: 'Changelog', display: 'children', theme: { pagination: false } },
}
