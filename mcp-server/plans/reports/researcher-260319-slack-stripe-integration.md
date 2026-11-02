# Integration Patterns: Slack OAuth + Stripe Subscriptions

## 1. Slack OAuth Workspace Installation

### OAuth v2 Flow (Recommended)
1. **Authorize**: Redirect to `https://slack.com/oauth/v2/authorize?client_id=...&scope=...&redirect_uri=...`
2. **Consent**: User installs to workspace; Slack redirects to `redirect_uri?code=...&state=...`
3. **Token Exchange**: POST to `oauth.v2.access` with `code`, `client_id`, `client_secret` → returns `bot.access_token` + bot user info

**Why**: v2 provides granular scopes; simpler security (two-factor: code + secret).

### Required OAuth Scopes
```
chat:write                 # Send messages + Block Kit blocks
channels:read              # List channels (optional)
groups:read                # List private channels (optional)
users:read                 # Resolve user IDs (optional)
```
No extra scopes for Block Kit—use `chat:write` with message payload containing blocks JSON.

### Token Storage (Security)
- **Per-org model**: Store `workspace_id`, `bot.access_token`, `bot.user_id` in DB (encrypted at rest)
- Rotate tokens via `oauth.v2.refresh` if implementing token rotation
- Revoke on org offboarding: Slack token invalidates automatically when user revokes app

### Interactivity Callback (Block Actions)
1. Set **Request URL** in Slack app config → your backend (HTTPS required for distributed apps; HTTP OK for single-workspace)
2. Slack POSTs `payload` (form-encoded JSON) to URL on button/select click
3. Handler **must** respond within 3 seconds (empty 200 OK) to prevent Slack timeout retry
4. Parse `payload` JSON for `type: "block_actions"`, `actions[]`, `trigger_id`, `response_url`
5. Use `response_url` (webhook) to send follow-up messages; use `chat.postMessage` for other channels

### Go Library: slack-go/slack
```go
import "github.com/slack-go/slack"

// OAuth v2
resp, err := api.GetOAuthV2Response(ctx, clientID, clientSecret, code, redirectURI)
botToken := resp.AccessToken
botUserID := resp.BotUserID

// Send message with Block Kit
api := slack.New(botToken)
err = api.PostMessage(channelID, slack.MsgOptionBlocks(
  // blocks JSON here
))

// Handle interactivity
payload := slack.InteractionCallback{}
json.Unmarshal([]byte(payloadStr), &payload)
if payload.Type == slack.InteractionTypeBlockActions {
  // Handle button clicks
}
```

---

## 2. Stripe Subscription Billing (Go + Next.js)

### Checkout Session Flow (Recommended)
1. **Frontend** (Next.js): POST `/api/checkout-session` with `priceId` (or `productId` + `quantity` for per-seat)
2. **Backend** (Go): Create Stripe `CheckoutSession` → return `sessionId`
3. **Frontend**: Redirect to Stripe Checkout URL (`stripe.redirectToCheckout(sessionId)` or hosted link)
4. **Stripe → Webhook**: On success, Stripe POSTs `checkout.session.completed` to backend
5. **Backend**: Verify webhook signature, fetch session, create/update subscription in DB

**Why**: Stripe handles PCI compliance; checkout hosted page, not custom form.

### Go stripe-go Library
```go
import "github.com/stripe/stripe-go/v84"
import "github.com/stripe/stripe-go/v84/checkout/session"

stripe.Key = stripeAPIKey

// Create checkout session for subscription
params := &stripe.CheckoutSessionParams{
  Mode:       stripe.String("subscription"),
  LineItems: []*stripe.CheckoutSessionLineItemParams{
    {
      Price:    stripe.String(priceID), // e.g., "price_xxx" for fixed plan
      Quantity: stripe.Int64(seatCount),    // for per-seat: quantity = user count
    },
  },
  SuccessURL: stripe.String("https://yourapp.com/billing?status=success"),
  CancelURL:  stripe.String("https://yourapp.com/billing?status=canceled"),
  Customer:   stripe.String(customerID), // optional; links to existing Stripe customer
}
sess, err := session.New(params)
// Return sess.ID to frontend
```

### Required Webhooks
```
checkout.session.completed        # Subscription created (process in DB)
customer.subscription.updated      # Quantity/plan changed (proration)
customer.subscription.deleted      # Canceled (revoke access)
invoice.payment_failed             # Auto-renewal failed (notify user)
invoice.paid                       # Renew succeeded (extend access)
customer.subscription.trial_will_end  # Trial 3 days out (optional)
```

**Webhook Handler**:
```go
import "github.com/stripe/stripe-go/v84/webhook"

event := stripe.Event{}
err := json.Unmarshal(body, &event)
if err = webhook.VerifySignature(body, sigHeader, endpointSecret); err != nil {
  return 400
}

switch event.Type {
case "checkout.session.completed":
  var session stripe.CheckoutSession
  event.DataObjectRaw.UnmarshalJSON(&session)
  // subscription ID in session.Subscription; activate user access

case "customer.subscription.deleted":
  var sub stripe.Subscription
  event.DataObjectRaw.UnmarshalJSON(&sub)
  // Revoke access for org
}
```

### Per-Seat Pricing
- Create a **price** with `recurring[usage_type]: "licensed"` in Stripe Dashboard
- On checkout: `Quantity = org_user_count`
- On user add/delete: Update subscription quantity via `subscription.Update()` → Stripe proration auto-calculates
- Prorated credit/debit appears on next invoice

### Customer Portal
- Generate portal session: `billingportal.Session.New()` → returns `url`
- User clicks link → manages payment methods, plan upgrades, cancellation
- **Webhooks alone don't show portal events**; portal updates subscriptions directly via Stripe API
- Portal changes trigger `customer.subscription.updated` webhook normally

### Organization-to-Stripe Mapping
- 1:1: `organization.stripe_customer_id` (store on first checkout)
- Create customer: `customers.New(params)` with metadata `org_id`
- Reuse customer on future checkouts (pass `customer_id` in checkout params)

### Next.js → Go Routing
Dashboard calls `/api/proxy/[...path]` (Next.js route handler) → Go backend `/api/v1/billing/checkout-session`

---

## Unresolved Questions
1. Does Slack workspace support multiple org installs (org-level vs workspace-level)?
2. Should per-seat dynamic updates use `subscription.Update()` or new checkout session?
3. How to handle Stripe trial period with approval workflow in valtag?
4. Should invoice.upcoming webhook auto-add seats, or manual user confirmation?

---

## Sources

### Slack OAuth & Interactivity
- [Slack OAuth v2 Installation](https://docs.slack.dev/authentication/installing-with-oauth/)
- [Slack OAuth Scopes](https://api.slack.com/scopes)
- [Handling Slack Interactivity](https://docs.slack.dev/interactivity/handling-user-interaction/)
- [slack-go/slack Library](https://github.com/slack-go/slack)

### Stripe Subscriptions & Billing
- [Stripe Checkout Build Subscriptions](https://docs.stripe.com/billing/subscriptions/build-subscriptions)
- [Stripe Webhooks for Subscriptions](https://docs.stripe.com/billing/subscriptions/webhooks)
- [Stripe Per-Seat Pricing](https://docs.stripe.com/subscriptions/pricing-models/per-seat-pricing)
- [stripe-go Library](https://github.com/stripe/stripe-go)
