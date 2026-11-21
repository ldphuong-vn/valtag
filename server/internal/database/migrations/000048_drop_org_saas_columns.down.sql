ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS plan VARCHAR(50),
    ADD COLUMN IF NOT EXISTS stripe_customer_id VARCHAR(255),
    ADD COLUMN IF NOT EXISTS stripe_subscription_id VARCHAR(255),
    ADD COLUMN IF NOT EXISTS plan_seats INT;
