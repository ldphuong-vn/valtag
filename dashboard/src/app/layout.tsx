import type { Metadata } from 'next'
import { Geist, Geist_Mono, Inter, Public_Sans } from 'next/font/google'
import './globals.css'
import { cn } from "@/lib/utils";

const publicSansHeading = Public_Sans({subsets:['latin'],variable:'--font-heading'});

const inter = Inter({subsets:['latin'],variable:'--font-sans'});

const geistSans = Geist({ subsets: ['latin'], variable: '--font-geist-sans' })
const geistMono = Geist_Mono({ subsets: ['latin'], variable: '--font-geist-mono' })

export const metadata: Metadata = {
  title: 'valtag - AI Secret Vault',
  description: 'Human-in-the-loop secret management for AI agents',
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={cn("dark", "font-sans", inter.variable, publicSansHeading.variable)}>
      <body className={`${geistSans.variable} ${geistMono.variable} min-h-screen bg-background antialiased font-sans`}>
        {children}
      </body>
    </html>
  )
}
