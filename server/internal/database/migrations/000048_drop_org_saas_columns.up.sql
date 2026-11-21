-- SaaS-era billing columns are retired: valtag is self-hosted software with
-- no plans to sell (2026-11-21). org → user relation stays in org_memberships.
ALTER TABLE organizations
    DROP COLUMN IF EXISTS plan,
    DROP COLUMN IF EXISTS stripe_customer_id,
    DROP COLUMN IF EXISTS stripe_subscription_id,
    DROP COLUMN IF EXISTS plan_seats;
