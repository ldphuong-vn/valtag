import Link from 'next/link'
import { Button } from '@/components/ui/button'
import { Shield } from 'lucide-react'

const navLinks = [
  { label: 'Home', href: '#' },
  { label: 'Features', href: '#features' },
  { label: 'Pricing', href: '#pricing' },
  { label: 'Docs', href: 'https://docs.valt.dev' },
]

export function LandingNavbar() {
  return (
    <nav className="fixed top-0 z-50 w-full" style={{ background: 'rgba(0,0,0,0.8)', backdropFilter: 'blur(20px)' }}>
      <div className="mx-auto flex h-16 max-w-6xl items-center justify-between px-4 sm:px-6">
        {/* Logo */}
        <Link href="/" className="flex items-center gap-2.5 text-lg font-semibold tracking-tight text-white">
          <div
            className="flex h-8 w-8 items-center justify-center rounded-lg"
            style={{ background: 'rgba(99,102,241,0.15)', border: '1px solid rgba(99,102,241,0.25)' }}
          >
            <Shield className="h-4 w-4" style={{ color: 'oklch(0.6 0.25 270)' }} />
          </div>
          valtag
        </Link>

        {/* Center links */}
        <div className="hidden items-center gap-8 text-sm md:flex">
          {navLinks.map((link) => (
            <a
              key={link.label}
              href={link.href}
              className="transition-colors duration-200 hover:text-white"
              style={{ color: 'rgba(255,255,255,0.5)' }}
            >
              {link.label}
            </a>
          ))}
        </div>

        {/* CTA */}
        <div className="flex items-center gap-3">
          <Link href="/login">
            <Button
              size="sm"
              className="rounded-full px-5 text-white"
              style={{
                background: 'oklch(0.5 0.22 265)',
                boxShadow: '0 0 20px -4px oklch(0.5 0.22 265 / 40%)',
              }}
            >
              Get Started
            </Button>
          </Link>
        </div>
      </div>

      {/* Gradient border bottom */}
      <div
        className="absolute inset-x-0 bottom-0 h-px"
        style={{
          background: 'linear-gradient(90deg, transparent, oklch(0.5 0.22 265 / 40%) 50%, transparent)',
        }}
      />
    </nav>
  )
}
