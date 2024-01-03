export default {
  '*': { theme: { timestamp: false } },
  index: {
    title: 'Home', type: 'page', display: 'hidden',
    theme: { layout: 'full', sidebar: false, toc: false, breadcrumb: false, pagination: false, copyPage: false },
  },
  releases: { title: 'Changelog', type: 'page', href: '/changelog/' },
  releaseHistory: { title: 'Changelog', type: 'separator' },
  changelog: { title: 'Changelog', display: 'children', theme: { pagination: false } },
}
