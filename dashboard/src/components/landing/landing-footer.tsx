import { Shield } from 'lucide-react'

const links = {
  Product: [
    { label: 'Features', href: '#features' },
    { label: 'Pricing', href: '#pricing' },
    { label: 'Documentation', href: 'https://docs.valt.dev' },
  ],
  Company: [
    { label: 'About', href: '#' },
    { label: 'Blog', href: '#' },
    { label: 'Contact', href: 'mailto:hello@valt.dev' },
  ],
  Legal: [
    { label: 'Terms of Service', href: '/terms' },
    { label: 'Privacy Policy', href: '/privacy' },
  ],
}

export function LandingFooter() {
  return (
    <footer className="relative px-4 py-12">
      {/* Gradient top border */}
      <div
        className="absolute inset-x-0 top-0 h-px"
        style={{ background: 'linear-gradient(90deg, transparent, rgba(255,255,255,0.08) 50%, transparent)' }}
      />

      <div className="mx-auto max-w-6xl">
        <div className="grid gap-8 sm:grid-cols-2 lg:grid-cols-4">
          {/* Brand */}
          <div>
            <div className="flex items-center gap-2.5 text-lg font-semibold tracking-tight text-white">
              <div
                className="flex h-8 w-8 items-center justify-center rounded-lg"
                style={{ background: 'rgba(99,102,241,0.15)', border: '1px solid rgba(99,102,241,0.25)' }}
              >
                <Shield className="h-4 w-4" style={{ color: 'oklch(0.6 0.25 270)' }} />
              </div>
              valtag
            </div>
            <p className="mt-3 text-sm leading-relaxed" style={{ color: 'rgba(255,255,255,0.4)' }}>
              MCP-native secret vault with human-in-the-loop approval for AI agents.
            </p>
          </div>

          {/* Link columns */}
          {Object.entries(links).map(([group, items]) => (
            <div key={group}>
              <h4 className="mb-3 text-sm font-semibold text-white">{group}</h4>
              <ul className="space-y-2.5">
                {items.map((item) => (
                  <li key={item.label}>
                    <a
                      href={item.href}
                      className="text-sm transition-colors duration-200 hover:text-white"
                      style={{ color: 'rgba(255,255,255,0.4)' }}
                    >
                      {item.label}
                    </a>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>

        <div
          className="mt-12 pt-6 text-center text-xs"
          style={{ borderTop: '1px solid rgba(255,255,255,0.06)', color: 'rgba(255,255,255,0.25)' }}
        >
          &copy; {new Date().getFullYear()} valtag. All rights reserved.
        </div>
      </div>
    </footer>
  )
}
