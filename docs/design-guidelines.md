# Design Guidelines

## Brand
- **Name**: valtag
- **Tagline**: AI Secret Vault with Human-in-the-Loop Approval
- **Tone**: Professional, security-focused, developer-friendly

## Color Palette
- Primary: Neutral/Dark (black, slate)
- Accent: Blue for actions, green for success, red for destructive
- Background: White (light mode), slate-950 (dark mode)

## Typography
- Font: System font stack (Inter for web)
- Headings: Bold, tracking-tight
- Body: Regular, readable line height

## Components
- Use shadcn/ui as component library
- Consistent spacing: 4px grid (p-1, p-2, p-4, p-6, p-8)
- Rounded corners: rounded-lg (8px)
- Shadows: subtle, shadow-sm for cards

## Layout
- Dashboard: sidebar navigation + main content area
- Auth pages: centered card layout
- Mobile: responsive, stacked layout
- Max content width: max-w-7xl

## Dark Mode
- Support dark mode via Tailwind dark: prefix
- Use CSS variables for theme colors
