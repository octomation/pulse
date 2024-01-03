import { Footer, Layout, Navbar } from 'nextra-theme-docs'
import { Head } from 'nextra/components'
import { getPageMap } from 'nextra/page-map'
import 'nextra-theme-docs/style.css'
import './globals.css'
import { siteUrl } from '../site.mjs'
import { groupReleases } from '../navigation.mjs'

export const metadata = {
  title: { default: 'pulse — All your activity in one view.', template: '%s · pulse' },
  description: 'The one point of view to all external activities, starting with GitHub.',
  metadataBase: new URL(siteUrl),
  icons: { icon: `${process.env.BASE_PATH || ''}/icon.svg` },
}

function withExportLinks(items) {
  return items.map(item => ({
    ...item,
    ...('frontMatter' in item && { href: item.href || `${item.route.replace(/\/$/, '')}/` }),
    ...(item.children && { children: withExportLinks(item.children) }),
  }))
}

export default async function RootLayout({ children }) {
  return (
    <html lang="en" dir="ltr" suppressHydrationWarning>
      <Head color={{ hue: 4, saturation: 68, lightness: { light: 42, dark: 68 } }} backgroundColor={{ light: 'rgb(250, 249, 246)', dark: 'rgb(20, 24, 23)' }} />
      <body>
        <Layout
          navbar={<Navbar logo={<span className="wordmark"><span className="brand-emoji" aria-hidden="true">📰</span>pulse</span>} projectLink="https://github.com/octomation/pulse" />}
          pageMap={withExportLinks(groupReleases(await getPageMap()))}
          docsRepositoryBase="https://github.com/octomation/pulse/tree/main/docs"
          footer={<Footer><div className="site-footer"><span><b>pulse</b> <span>The one point of view to all external activities.</span></span><span>MIT © {new Date().getFullYear()} <a href="https://github.com/octolab">OctoLab</a></span></div></Footer>}
          search={null}
          copyPageButton={false}
          feedback={{ content: 'Something unclear?', link: 'https://github.com/octomation/pulse/issues/new' }}
          nextThemes={{ defaultTheme: 'system' }}
        >
          {children}
        </Layout>
      </body>
    </html>
  )
}
