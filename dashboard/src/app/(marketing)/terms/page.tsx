import type { Metadata } from 'next'
import Link from 'next/link'

export const metadata: Metadata = {
  title: 'Terms of Service – valtag',
  description: 'Terms of Service for valtag, the MCP-native secret vault for AI agents.',
}

export default function TermsPage() {
  return (
    <article className="prose prose-neutral dark:prose-invert max-w-none">
      <h1>Terms of Service</h1>
      <p className="lead">
        Last updated: <time dateTime="2026-03-24">March 24, 2026</time>
      </p>
      <p>
        These Terms of Service (&quot;Terms&quot;) govern your access to and use of valtag
        (&quot;Service&quot;), operated by Turbo AI (&quot;Company&quot;, &quot;we&quot;,
        &quot;us&quot;, or &quot;our&quot;), accessible at{' '}
        <a href="https://valt.turbo.ai.vn">valt.turbo.ai.vn</a>. By creating an account or
        using the Service, you agree to be bound by these Terms.
      </p>

      {/* 1 */}
      <h2>1. Description of Service</h2>
      <p>
        valtag is a secret management platform designed for AI agents and their operators. The
        Service provides:
      </p>
      <ul>
        <li>Encrypted storage of API keys, credentials, and other secrets.</li>
        <li>
          Human-in-the-loop approval workflows so operators can review and authorise secret
          access requests made by AI agents via the MCP protocol.
        </li>
        <li>Audit logging, dynamic secrets, and role-based access control.</li>
        <li>Integrations with third-party notification channels (Slack, Telegram, email).</li>
      </ul>

      {/* 2 */}
      <h2>2. Account Registration</h2>
      <p>
        You must register for an account to use the Service. You agree to provide accurate,
        current, and complete information and to keep that information up to date. You are
        responsible for maintaining the confidentiality of your password and for all activity
        that occurs under your account. Notify us immediately at{' '}
        <a href="mailto:hello@valt.turbo.ai.vn">hello@valt.turbo.ai.vn</a> if you suspect
        unauthorised access.
      </p>

      {/* 3 */}
      <h2>3. Acceptable Use</h2>
      <p>You agree <strong>not</strong> to:</p>
      <ul>
        <li>Use the Service to store credentials that facilitate illegal activity.</li>
        <li>Attempt to reverse-engineer, probe, or exploit the Service infrastructure.</li>
        <li>Share your account credentials with third parties.</li>
        <li>Upload malicious code, malware, or content that violates applicable law.</li>
        <li>
          Circumvent rate limits, access controls, or billing restrictions through automation or
          other means.
        </li>
        <li>Resell or sublicense the Service without our prior written consent.</li>
      </ul>
      <p>
        We reserve the right to suspend or terminate accounts that violate these restrictions
        without prior notice.
      </p>

      {/* 4 */}
      <h2>4. Data Handling and Security</h2>
      <p>
        Secret values are encrypted client-side before transmission using AES-256-GCM envelope
        encryption. The Company does not have access to plaintext secret values (&quot;zero-knowledge
        architecture&quot;). Metadata (secret names, tags, access logs) is stored in our
        database and is visible to the Company for operational purposes.
      </p>
      <p>
        While we implement industry-standard security measures, no system is perfectly secure.
        You are responsible for the sensitivity and classification of data you store in the
        Service.
      </p>

      {/* 5 */}
      <h2>5. Billing and Payments</h2>
      <p>
        Paid plans are billed on a subscription basis (monthly or annually) through Stripe, our
        third-party payment processor. By subscribing you authorise Stripe to charge your
        payment method on a recurring basis.
      </p>
      <ul>
        <li>
          <strong>Cancellation:</strong> You may cancel your subscription at any time from the
          billing settings. Access continues until the end of the current billing period; no
          partial refunds are issued.
        </li>
        <li>
          <strong>Price changes:</strong> We will provide at least 30 days&apos; notice before
          increasing subscription prices.
        </li>
        <li>
          <strong>Taxes:</strong> Prices are exclusive of applicable taxes, which are your
          responsibility.
        </li>
        <li>
          <strong>Free tier:</strong> We may offer a free tier with usage limits. Limits are
          subject to change with reasonable notice.
        </li>
      </ul>

      {/* 6 */}
      <h2>6. Intellectual Property</h2>
      <p>
        The Service, including its software, design, trademarks, and documentation, is owned by
        Turbo AI and protected by applicable intellectual property laws. Nothing in these Terms
        transfers ownership of any intellectual property to you. You retain all rights to the
        data and secrets you store in the Service.
      </p>

      {/* 7 */}
      <h2>7. Limitation of Liability</h2>
      <p>
        To the maximum extent permitted by law, Turbo AI and its affiliates, officers, employees,
        and agents shall not be liable for any indirect, incidental, special, consequential, or
        punitive damages, including loss of profits, data, or business opportunities, arising out
        of or in connection with the Service, even if advised of the possibility of such damages.
      </p>
      <p>
        Our total aggregate liability to you for any claim arising under these Terms shall not
        exceed the amounts you paid to us in the twelve (12) months preceding the claim.
      </p>

      {/* 8 */}
      <h2>8. Disclaimer of Warranties</h2>
      <p>
        The Service is provided &quot;as is&quot; and &quot;as available&quot; without warranties of
        any kind, express or implied, including but not limited to warranties of merchantability,
        fitness for a particular purpose, and non-infringement. We do not warrant that the Service
        will be uninterrupted, error-free, or free of harmful components.
      </p>

      {/* 9 */}
      <h2>9. Termination</h2>
      <p>
        Either party may terminate the agreement at any time. Upon termination, your right to
        access the Service ceases immediately. We will retain your encrypted data for 30 days
        after termination to allow export, after which it will be permanently deleted. You may
        request immediate deletion by contacting{' '}
        <a href="mailto:hello@valt.turbo.ai.vn">hello@valt.turbo.ai.vn</a>.
      </p>

      {/* 10 */}
      <h2>10. Changes to Terms</h2>
      <p>
        We may update these Terms from time to time. Material changes will be communicated via
        email or an in-app notification at least 14 days before taking effect. Continued use of
        the Service after the effective date constitutes acceptance of the revised Terms.
      </p>

      {/* 11 */}
      <h2>11. Governing Law</h2>
      <p>
        These Terms are governed by and construed in accordance with the laws of the Socialist
        Republic of Vietnam, without regard to conflict-of-law principles. Any disputes shall be
        resolved in the competent courts of Ho Chi Minh City, Vietnam.
      </p>

      {/* 12 */}
      <h2>12. Contact</h2>
      <p>
        For questions about these Terms, contact us at{' '}
        <a href="mailto:hello@valt.turbo.ai.vn">hello@valt.turbo.ai.vn</a> or write to:
      </p>
      <address className="not-italic">
        Turbo AI<br />
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
