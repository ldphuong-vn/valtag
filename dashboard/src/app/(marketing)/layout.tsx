import Link from 'next/link'
import { Shield } from 'lucide-react'

export default function MarketingLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen bg-background text-foreground">
      {/* Minimal nav */}
      <nav className="border-b border-border/40 bg-background/80 backdrop-blur-md sticky top-0 z-50">
        <div className="mx-auto flex h-14 max-w-4xl items-center justify-between px-4 sm:px-6">
          <Link href="/" className="flex items-center gap-2 text-base font-semibold">
            <Shield className="h-4 w-4 text-primary" />
            valtag
          </Link>
          <div className="flex items-center gap-4 text-sm text-muted-foreground">
            <Link href="/terms" className="transition-colors hover:text-foreground">Terms</Link>
            <Link href="/privacy" className="transition-colors hover:text-foreground">Privacy</Link>
          </div>
        </div>
      </nav>

      {/* Content */}
      <main className="mx-auto max-w-4xl px-4 py-12 sm:px-6">
        {children}
      </main>

      {/* Footer */}
      <footer className="border-t border-border/40 py-6 text-center text-xs text-muted-foreground">
        &copy; {new Date().getFullYear()} Turbo AI. All rights reserved.
        {' · '}
        <Link href="/terms" className="hover:text-foreground transition-colors">Terms</Link>
        {' · '}
        <Link href="/privacy" className="hover:text-foreground transition-colors">Privacy</Link>
      </footer>
    </div>
  )
}
