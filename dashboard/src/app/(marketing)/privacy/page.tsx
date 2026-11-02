import type { Metadata } from 'next'
import Link from 'next/link'

export const metadata: Metadata = {
  title: 'Privacy Policy – valtag',
  description: 'Privacy Policy for valtag, the MCP-native secret vault for AI agents.',
}

export default function PrivacyPage() {
  return (
    <article className="prose prose-neutral dark:prose-invert max-w-none">
      <h1>Privacy Policy</h1>
      <p className="lead">
        Last updated: <time dateTime="2026-03-24">March 24, 2026</time>
      </p>
      <p>
        This Privacy Policy explains how Turbo AI (&quot;Company&quot;, &quot;we&quot;,
        &quot;us&quot;, or &quot;our&quot;) collects, uses, and protects information when you
        use valtag (&quot;Service&quot;) at{' '}
        <a href="https://valt.turbo.ai.vn">valt.turbo.ai.vn</a>.
      </p>

      {/* 1 */}
      <h2>1. Information We Collect</h2>

      <h3>1.1 Account Information</h3>
      <ul>
        <li>Email address and display name (required for registration).</li>
        <li>Password hash (Argon2id; we never store plaintext passwords).</li>
        <li>Google OAuth profile data if you sign in with Google (email, name, avatar URL).</li>
        <li>Organisation name and billing contact details.</li>
      </ul>

      <h3>1.2 Usage and Operational Data</h3>
      <ul>
        <li>Secret <em>metadata</em>: names, tags, project associations, created/updated timestamps.</li>
        <li>
          Access request logs: which agent requested which secret, timestamps, approval decisions.
          This data forms the tamper-evident audit log.
        </li>
        <li>Agent identifiers and API token hashes (not plaintext tokens).</li>
        <li>Plan usage counters (number of secrets, requests, agents).</li>
      </ul>

      <h3>1.3 What We Do NOT Collect</h3>
      <p>
        <strong>Secret values are never visible to us.</strong> Values are encrypted client-side
        with AES-256-GCM envelope encryption before leaving your browser. The encrypted blob is
        stored in object storage; we hold only the wrapped Data Encryption Key (DEK) alongside
        the ciphertext. Without your master key the plaintext cannot be recovered by us.
      </p>

      <h3>1.4 Technical Data</h3>
      <ul>
        <li>IP addresses and HTTP request logs retained for up to 30 days for security purposes.</li>
        <li>Browser/client user-agent strings.</li>
        <li>Error and crash reports (no secret values included).</li>
      </ul>

      {/* 2 */}
      <h2>2. How We Use Your Data</h2>
      <ul>
        <li>Provide, operate, and improve the Service.</li>
        <li>Authenticate users and enforce access controls.</li>
        <li>Send transactional emails (approval notifications, billing receipts, security alerts).</li>
        <li>Enforce plan limits and prevent abuse.</li>
        <li>Comply with legal obligations and respond to lawful requests.</li>
        <li>
          Aggregate, anonymised analytics to understand feature usage (no individual profiling).
        </li>
      </ul>
      <p>We do not sell your personal data to third parties.</p>

      {/* 3 */}
      <h2>3. Encryption and Security</h2>
      <p>
        All data in transit is protected by TLS 1.2+. At rest:
      </p>
      <ul>
        <li>
          <strong>Secret values:</strong> AES-256-GCM, encrypted client-side. DEKs are
          envelope-encrypted with a master key held only in our server environment variables;
          even our database administrators cannot read plaintext values.
        </li>
        <li>
          <strong>Dynamic secret provider configs:</strong> AES-256-GCM server-side encryption
          using the same master key.
        </li>
        <li>
          <strong>Passwords:</strong> Argon2id with per-user salts, never stored in recoverable
          form.
        </li>
        <li>
          <strong>Auth tokens:</strong> RS256-signed JWTs; short-lived access tokens with
          httpOnly, Secure, SameSite=Lax cookies.
        </li>
      </ul>

      {/* 4 */}
      <h2>4. Third-Party Services</h2>
      <p>We share limited data with the following sub-processors:</p>
      <ul>
        <li>
          <strong>Stripe</strong> — payment processing. Stripe receives your billing information
          directly; we store only a Stripe Customer ID and subscription status. Stripe&apos;s
          privacy policy: <a href="https://stripe.com/privacy" target="_blank" rel="noreferrer">stripe.com/privacy</a>.
        </li>
        <li>
          <strong>Google OAuth</strong> — optional sign-in. If you use &quot;Sign in with
          Google&quot;, Google shares your email, name, and avatar URL with us per your Google
          account privacy settings.
        </li>
        <li>
          <strong>Slack / Telegram</strong> — optional notification channels. If you connect
          these integrations, we send approval notification messages to your configured channel.
          No secret values are included in notifications.
        </li>
        <li>
          <strong>SMTP provider</strong> — transactional email delivery (approval alerts,
          receipts). Email content contains metadata only, never secret values.
        </li>
      </ul>

      {/* 5 */}
      <h2>5. Cookies</h2>
      <p>We use a minimal set of cookies:</p>
      <ul>
        <li>
          <strong>valt_access_token</strong> — httpOnly, Secure, SameSite=Lax session cookie
          containing a signed JWT. Required for authentication; no analytics or tracking purpose.
        </li>
      </ul>
      <p>
        We do not use advertising cookies, third-party trackers, or analytics cookies. No cookie
        consent banner is required beyond this disclosure.
      </p>

      {/* 6 */}
      <h2>6. Data Retention</h2>
      <ul>
        <li>
          <strong>Active accounts:</strong> data is retained for the duration of your
          subscription plus 30 days after cancellation to allow data export.
        </li>
        <li>
          <strong>Audit logs:</strong> retained for 12 months by default; configurable per
          organisation on enterprise plans.
        </li>
        <li>
          <strong>HTTP/IP logs:</strong> 30 days for security monitoring.
        </li>
        <li>
          <strong>Deleted secrets:</strong> permanently removed within 24 hours of deletion.
        </li>
        <li>
          <strong>After account deletion:</strong> all personal data purged within 30 days
          except where retention is required by law.
        </li>
      </ul>

      {/* 7 */}
      <h2>7. Your Rights</h2>
      <p>
        Subject to applicable law, you have the right to:
      </p>
      <ul>
        <li><strong>Access</strong> — request a copy of the personal data we hold about you.</li>
        <li>
          <strong>Rectification</strong> — correct inaccurate personal data via your account
          settings or by contacting us.
        </li>
        <li>
          <strong>Erasure</strong> — request deletion of your account and associated data.
        </li>
        <li>
          <strong>Export</strong> — download your secrets (encrypted) and audit logs from the
          dashboard before account deletion.
        </li>
        <li>
          <strong>Restriction</strong> — request that we restrict processing of your data in
          certain circumstances.
        </li>
        <li>
          <strong>Objection</strong> — object to processing based on legitimate interests.
        </li>
      </ul>
      <p>
        To exercise any right, email{' '}
        <a href="mailto:privacy@valt.turbo.ai.vn">privacy@valt.turbo.ai.vn</a>. We will respond
        within 30 days.
      </p>

      {/* 8 */}
      <h2>8. Data Transfers</h2>
      <p>
        Our servers are currently located in Southeast Asia. If you access the Service from
        outside Vietnam, your data may be transferred internationally. We apply appropriate
        safeguards consistent with applicable data protection law.
      </p>

      {/* 9 */}
      <h2>9. Children</h2>
      <p>
        The Service is not directed to children under 16. We do not knowingly collect personal
        data from anyone under 16. If you believe we have done so inadvertently, contact us for
        immediate deletion.
      </p>

      {/* 10 */}
      <h2>10. Changes to This Policy</h2>
      <p>
        We may update this Privacy Policy periodically. Material changes will be communicated
        via email at least 14 days before taking effect. The &quot;Last updated&quot; date at
        the top reflects the most recent revision.
      </p>

      {/* 11 */}
      <h2>11. Contact</h2>
      <p>
        For privacy enquiries or to exercise your rights:
      </p>
      <address className="not-italic">
        <strong>Turbo AI — Privacy</strong><br />
        <a href="mailto:privacy@valt.turbo.ai.vn">privacy@valt.turbo.ai.vn</a><br />
        Ho Chi Minh City, Vietnam
      </address>

      <hr />
      <p>
        <Link href="/" className="text-sm text-muted-foreground hover:text-foreground transition-colors">
          &larr; Back to home
        </Link>
      </p>
    </article>
  )
}
