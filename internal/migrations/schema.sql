BEGIN;

CREATE SCHEMA IF NOT EXISTS public;
SET search_path TO public;

CREATE EXTENSION IF NOT EXISTS btree_gist;

-- ============================================================
-- ENUMS
-- ============================================================

CREATE TYPE user_role AS ENUM ('owner', 'barber');

CREATE TYPE appointment_status AS ENUM (
  'scheduled',
  'awaiting_payment',
  'completed',
  'cancelled',
  'no_show'
);

CREATE TYPE appointment_created_by AS ENUM (
  'client',
  'barber'
);

CREATE TYPE payment_intent_type AS ENUM (
  'pay_later',
  'paid'
);

CREATE TYPE no_show_source_type AS ENUM (
  'auto',
  'manual'
);

CREATE TYPE payment_status AS ENUM (
  'pending',
  'paid',
  'expired'
);

CREATE TYPE client_category AS ENUM (
  'new',
  'regular',
  'trusted',
  'at_risk'
);

CREATE TYPE category_source_type AS ENUM (
  'auto',
  'manual'
);

CREATE TYPE payment_requirement AS ENUM (
  'mandatory',
  'optional',
  'none'
);

CREATE TYPE order_type AS ENUM (
  'product'
);

CREATE TYPE order_status AS ENUM (
  'pending',
  'paid',
  'cancelled'
);

CREATE TYPE subscription_status AS ENUM (
  'active',
  'cancelled',
  'expired',
  'pending_payment'
);

CREATE TYPE coverage_status AS ENUM (
  'none',
  'covered',
  'not_covered_service',
  'not_covered_exhausted',
  'not_covered_expired'
);

-- ============================================================
-- updated_at trigger
-- ============================================================

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ============================================================
-- BARBERSHOPS
-- ============================================================

CREATE TABLE barbershops (
  id            BIGSERIAL    PRIMARY KEY,
  name          VARCHAR(100) NOT NULL,
  slug          VARCHAR(100) NOT NULL UNIQUE,
  phone         VARCHAR(20),
  address       VARCHAR(255),
  email         VARCHAR(255),

  -- Structured address (006)
  cep           VARCHAR(9),
  street_name   VARCHAR(255),
  street_number VARCHAR(20),
  complement    VARCHAR(100),
  neighborhood  VARCHAR(100),
  city          VARCHAR(100),
  state         VARCHAR(2),

  min_advance_minutes       INTEGER NOT NULL DEFAULT 120,
  schedule_tolerance_minutes INTEGER NOT NULL DEFAULT 0,
  timezone                  VARCHAR(64) NOT NULL DEFAULT 'America/Sao_Paulo',
  photo_url                 TEXT,

  -- SaaS billing
  status TEXT NOT NULL DEFAULT 'trial'
    CHECK (status IN ('pending_payment', 'trial', 'active', 'inactive', 'suspended')),
  trial_ends_at          TIMESTAMPTZ,
  subscription_expires_at TIMESTAMPTZ,

  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TRIGGER trg_barbershops_updated
BEFORE UPDATE ON barbershops
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- BARBERSHOP PAYMENT CONFIG
-- ============================================================

CREATE TABLE barbershop_payment_configs (
  id            BIGSERIAL PRIMARY KEY,
  barbershop_id BIGINT NOT NULL UNIQUE REFERENCES barbershops(id) ON DELETE CASCADE,
  default_requirement   payment_requirement NOT NULL,
  pix_expiration_minutes INTEGER NOT NULL DEFAULT 15,
  mp_access_token VARCHAR(255),
  mp_public_key   VARCHAR(255),
  accept_cash    BOOLEAN NOT NULL DEFAULT false,
  accept_pix     BOOLEAN NOT NULL DEFAULT false,
  accept_credit  BOOLEAN NOT NULL DEFAULT false,
  accept_debit   BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

COMMENT ON COLUMN barbershop_payment_configs.pix_expiration_minutes IS
'PIX expiration window in minutes (used to set payments.expires_at).';

CREATE TRIGGER trg_payment_config_updated
BEFORE UPDATE ON barbershop_payment_configs
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- USERS
-- ============================================================

CREATE TABLE users (
  id            BIGSERIAL    PRIMARY KEY,
  barbershop_id BIGINT REFERENCES barbershops(id) ON DELETE SET NULL,
  name          VARCHAR(100) NOT NULL,
  email         VARCHAR(100) NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  phone         VARCHAR(20),
  role          user_role NOT NULL DEFAULT 'owner',
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_users_barbershop ON users(barbershop_id);

CREATE TRIGGER trg_users_updated
BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- PASSWORD RESET TOKENS (005)
-- ============================================================

CREATE TABLE password_reset_tokens (
  id         BIGSERIAL    PRIMARY KEY,
  user_id    BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token      VARCHAR(64)  NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ  NOT NULL,
  used_at    TIMESTAMPTZ,
  created_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_password_reset_tokens_token ON password_reset_tokens(token);

-- ============================================================
-- CLIENTS
-- ============================================================

CREATE TABLE clients (
  id            BIGSERIAL    PRIMARY KEY,
  barbershop_id BIGINT REFERENCES barbershops(id) ON DELETE SET NULL,
  name          VARCHAR(100) NOT NULL,
  phone         VARCHAR(20),
  email         VARCHAR(100),
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_clients_barbershop        ON clients(barbershop_id);
CREATE INDEX idx_clients_phone             ON clients(barbershop_id, phone);
CREATE INDEX idx_clients_name_lower        ON clients(barbershop_id, LOWER(name::text));

CREATE TRIGGER trg_clients_updated
BEFORE UPDATE ON clients
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- CLIENT METRICS
-- ============================================================

CREATE TABLE client_metrics (
  client_id     BIGINT NOT NULL,
  barbershop_id BIGINT NOT NULL,

  total_appointments      INTEGER NOT NULL DEFAULT 0,
  completed_appointments  INTEGER NOT NULL DEFAULT 0,
  cancelled_appointments  INTEGER NOT NULL DEFAULT 0,
  no_show_appointments    INTEGER NOT NULL DEFAULT 0,

  rescheduled_appointments      INTEGER NOT NULL DEFAULT 0,
  late_cancelled_appointments   INTEGER NOT NULL DEFAULT 0,
  late_rescheduled_appointments INTEGER NOT NULL DEFAULT 0,

  total_spent BIGINT NOT NULL DEFAULT 0 CHECK (total_spent >= 0),

  first_appointment_at       TIMESTAMPTZ,
  last_appointment_at        TIMESTAMPTZ,
  last_completed_at          TIMESTAMPTZ,
  last_canceled_at           TIMESTAMPTZ,
  last_no_show_at            TIMESTAMPTZ,
  last_late_canceled_at      TIMESTAMPTZ,
  last_late_rescheduled_at   TIMESTAMPTZ,

  category        client_category    NOT NULL DEFAULT 'new',
  category_source category_source_type NOT NULL DEFAULT 'auto',
  manual_category_expires_at TIMESTAMPTZ DEFAULT NULL,

  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now(),

  PRIMARY KEY (client_id, barbershop_id)
);

COMMENT ON COLUMN client_metrics.total_spent IS 'Money stored in CENTS (int64).';
COMMENT ON COLUMN client_metrics.manual_category_expires_at IS
'When set and category_source is ''manual'', the override expires at this timestamp
and auto-classification resumes on the next metric update. NULL = permanent override.';

-- ============================================================
-- CLIENT CRM CATEGORIES
-- ============================================================

CREATE TABLE client_crm_categories (
  barbershop_id BIGINT NOT NULL REFERENCES barbershops(id) ON DELETE CASCADE,
  client_id     BIGINT NOT NULL REFERENCES clients(id)    ON DELETE CASCADE,
  category      client_category NOT NULL DEFAULT 'new',
  updated_at    TIMESTAMPTZ     NOT NULL DEFAULT now(),
  PRIMARY KEY (barbershop_id, client_id)
);

CREATE INDEX idx_client_crm_categories_barbershop ON client_crm_categories(barbershop_id);

-- ============================================================
-- SERVICE CATEGORIES
-- ============================================================

CREATE TABLE service_categories (
  id            BIGSERIAL    PRIMARY KEY,
  barbershop_id BIGINT       NOT NULL REFERENCES barbershops(id) ON UPDATE CASCADE ON DELETE CASCADE,
  name          VARCHAR(100) NOT NULL,
  created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_service_categories_barbershop ON service_categories(barbershop_id);

CREATE TRIGGER trg_service_categories_updated
BEFORE UPDATE ON service_categories
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- BARBERSHOP SERVICES
-- ============================================================

CREATE TABLE barbershop_services (
  id            BIGSERIAL    PRIMARY KEY,
  barbershop_id BIGINT       NOT NULL REFERENCES barbershops(id) ON DELETE CASCADE,
  name          VARCHAR(100) NOT NULL,
  description   VARCHAR(255),
  duration_min  INTEGER,
  price         BIGINT       NOT NULL CHECK (price >= 0),
  active        BOOLEAN      DEFAULT true,
  category      VARCHAR(50),
  category_id   BIGINT       REFERENCES service_categories(id) ON UPDATE CASCADE ON DELETE SET NULL,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

COMMENT ON COLUMN barbershop_services.price IS 'Service price stored in CENTS (int64).';

CREATE INDEX idx_services_barbershop          ON barbershop_services(barbershop_id);
CREATE INDEX idx_barbershop_services_category ON barbershop_services(category_id);

CREATE TRIGGER trg_services_updated
BEFORE UPDATE ON barbershop_services
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- SERVICE IMAGES
-- ============================================================

CREATE TABLE service_images (
  id                    SERIAL       PRIMARY KEY,
  barbershop_service_id BIGINT       NOT NULL REFERENCES barbershop_services(id) ON DELETE CASCADE,
  url                   TEXT         NOT NULL,
  position              SMALLINT     NOT NULL DEFAULT 0,
  created_at            TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_service_images_service ON service_images(barbershop_service_id);

-- ============================================================
-- PRODUCTS
-- ============================================================

CREATE TABLE products (
  id            BIGSERIAL    PRIMARY KEY,
  barbershop_id BIGINT       NOT NULL REFERENCES barbershops(id) ON DELETE CASCADE,
  name          VARCHAR(100) NOT NULL,
  description   VARCHAR(255),
  category      VARCHAR(50),
  price         BIGINT       NOT NULL CHECK (price >= 0),
  stock         INTEGER      NOT NULL DEFAULT 0 CHECK (stock >= 0),
  active        BOOLEAN      DEFAULT true,
  online_visible BOOLEAN     NOT NULL DEFAULT false,
  image_url     TEXT,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

COMMENT ON COLUMN products.price IS 'Product price stored in CENTS (int64).';
COMMENT ON COLUMN products.online_visible IS 'Whether the product appears in the public catalog/store.';

CREATE INDEX idx_products_barbershop ON products(barbershop_id);

CREATE TRIGGER trg_products_updated
BEFORE UPDATE ON products
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- SERVICE SUGGESTED PRODUCTS
-- ============================================================

CREATE TABLE service_suggested_products (
  id            BIGSERIAL PRIMARY KEY,
  barbershop_id BIGINT    NOT NULL REFERENCES barbershops(id)       ON DELETE CASCADE,
  service_id    BIGINT    NOT NULL REFERENCES barbershop_services(id) ON DELETE CASCADE,
  product_id    BIGINT    NOT NULL REFERENCES products(id)          ON DELETE CASCADE,
  active        BOOLEAN   NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now(),
  UNIQUE(barbershop_id, service_id)
);

CREATE INDEX idx_ssp_service    ON service_suggested_products(service_id);
CREATE INDEX idx_ssp_product    ON service_suggested_products(product_id);
CREATE INDEX idx_ssp_barbershop ON service_suggested_products(barbershop_id);
CREATE INDEX idx_ssp_barbershop_service_active
  ON service_suggested_products(barbershop_id, service_id)
  WHERE active = true;

CREATE TRIGGER trg_service_suggested_products_updated
BEFORE UPDATE ON service_suggested_products
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- PLANS
-- ============================================================

CREATE TABLE plans (
  id                  BIGSERIAL PRIMARY KEY,
  barbershop_id       BIGINT    NOT NULL REFERENCES barbershops(id) ON DELETE CASCADE,
  name                VARCHAR(100) NOT NULL,
  monthly_price_cents BIGINT    NOT NULL CHECK (monthly_price_cents >= 0),
  duration_days       INTEGER   NOT NULL CHECK (duration_days > 0),
  cuts_included       INTEGER   NOT NULL CHECK (cuts_included >= 0),
  discount_percent    INTEGER   NOT NULL CHECK (discount_percent BETWEEN 0 AND 100),
  active              BOOLEAN   NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

COMMENT ON COLUMN plans.duration_days IS 'Plan duration in days. Used to calculate subscription current_period_end.';

CREATE INDEX idx_plans_barbershop ON plans(barbershop_id);

CREATE TRIGGER trg_plans_updated
BEFORE UPDATE ON plans
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- PLAN SERVICES (pivot)
-- ============================================================

CREATE TABLE plan_services (
  id         BIGSERIAL PRIMARY KEY,
  plan_id    BIGINT NOT NULL REFERENCES plans(id)              ON DELETE CASCADE,
  service_id BIGINT NOT NULL REFERENCES barbershop_services(id) ON DELETE CASCADE,
  UNIQUE(plan_id, service_id)
);

CREATE INDEX idx_plan_services_plan ON plan_services(plan_id);

-- ============================================================
-- PLAN CATEGORIES (pivot)
-- ============================================================

CREATE TABLE plan_categories (
  plan_id     BIGINT NOT NULL REFERENCES plans(id)              ON DELETE CASCADE,
  category_id BIGINT NOT NULL REFERENCES service_categories(id) ON DELETE CASCADE,
  PRIMARY KEY (plan_id, category_id)
);

CREATE INDEX idx_plan_categories_plan ON plan_categories(plan_id);

-- ============================================================
-- SUBSCRIPTIONS
-- ============================================================

CREATE TABLE subscriptions (
  id                   BIGSERIAL          PRIMARY KEY,
  barbershop_id        BIGINT             NOT NULL REFERENCES barbershops(id) ON DELETE CASCADE,
  client_id            BIGINT             NOT NULL REFERENCES clients(id)    ON DELETE CASCADE,
  plan_id              BIGINT             NOT NULL REFERENCES plans(id)      ON DELETE RESTRICT,
  status               subscription_status NOT NULL,
  current_period_start TIMESTAMPTZ        NOT NULL,
  current_period_end   TIMESTAMPTZ        NOT NULL,
  cuts_used_in_period     INTEGER NOT NULL DEFAULT 0 CHECK (cuts_used_in_period >= 0),
  cuts_reserved_in_period INTEGER NOT NULL DEFAULT 0 CHECK (cuts_reserved_in_period >= 0),
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_subscriptions_active ON subscriptions(barbershop_id, client_id, status);
CREATE INDEX idx_subscriptions_active_client
  ON subscriptions(barbershop_id, client_id, status)
  WHERE status = 'active';

CREATE UNIQUE INDEX uq_subscriptions_one_active_per_client_shop
  ON subscriptions(barbershop_id, client_id)
  WHERE status = 'active';

CREATE UNIQUE INDEX uq_subscriptions_one_pending_per_client_shop
  ON subscriptions(barbershop_id, client_id)
  WHERE status = 'pending_payment';

CREATE TRIGGER trg_subscriptions_updated
BEFORE UPDATE ON subscriptions
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- APPOINTMENTS
-- ============================================================

CREATE TABLE appointments (
  id                BIGSERIAL          PRIMARY KEY,
  barbershop_id     BIGINT REFERENCES barbershops(id)      ON DELETE SET NULL,
  barber_id         BIGINT REFERENCES users(id)            ON DELETE SET NULL,
  client_id         BIGINT REFERENCES clients(id)          ON DELETE SET NULL,
  barber_product_id BIGINT REFERENCES barbershop_services(id) ON DELETE SET NULL,
  subscription_id   BIGINT REFERENCES subscriptions(id)    ON DELETE SET NULL,

  start_time     TIMESTAMPTZ NOT NULL,
  end_time       TIMESTAMPTZ NOT NULL,
  status         appointment_status    NOT NULL DEFAULT 'scheduled',
  created_by     appointment_created_by NOT NULL DEFAULT 'client',
  payment_intent payment_intent_type   NOT NULL DEFAULT 'pay_later',
  coverage_status coverage_status      NOT NULL DEFAULT 'none',
  reserved_subscription_cut BOOLEAN    NOT NULL DEFAULT false,

  notes            VARCHAR(255),
  cancelled_at     TIMESTAMPTZ,
  completed_at     TIMESTAMPTZ,
  no_show_at       TIMESTAMPTZ,
  no_show_source   no_show_source_type,
  reschedule_count INTEGER NOT NULL DEFAULT 0,

  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_appointments_barber_time       ON appointments(barber_id, start_time);
CREATE INDEX idx_appointments_barbershop_status ON appointments(barbershop_id, status);
CREATE INDEX idx_appointments_barbershop_service ON appointments(barbershop_id, barber_product_id);
CREATE INDEX idx_appointments_barbershop_client  ON appointments(barbershop_id, client_id);
CREATE INDEX idx_appointments_barbershop_start   ON appointments(barbershop_id, start_time);

CREATE UNIQUE INDEX unique_barber_slot_active
  ON appointments(barbershop_id, barber_id, start_time)
  WHERE status IN ('scheduled', 'awaiting_payment');

CREATE TRIGGER trg_appointments_updated
BEFORE UPDATE ON appointments
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- ORDERS
-- ============================================================

CREATE TABLE orders (
  id            BIGSERIAL   PRIMARY KEY,
  barbershop_id BIGINT      NOT NULL REFERENCES barbershops(id) ON DELETE CASCADE,
  client_id     BIGINT      REFERENCES clients(id) ON DELETE SET NULL,
  type          order_type  NOT NULL,
  status        order_status NOT NULL DEFAULT 'pending',
  subtotal_amount BIGINT NOT NULL DEFAULT 0 CHECK (subtotal_amount >= 0),
  discount_amount BIGINT NOT NULL DEFAULT 0 CHECK (discount_amount >= 0),
  total_amount    BIGINT NOT NULL DEFAULT 0 CHECK (total_amount >= 0),
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

COMMENT ON COLUMN orders.subtotal_amount IS 'Order subtotal stored in CENTS (int64).';
COMMENT ON COLUMN orders.discount_amount IS 'Order discount stored in CENTS (int64).';
COMMENT ON COLUMN orders.total_amount    IS 'Order total stored in CENTS (int64).';

CREATE INDEX idx_orders_barbershop      ON orders(barbershop_id);
CREATE INDEX idx_orders_client          ON orders(client_id);
CREATE INDEX idx_orders_barbershop_date ON orders(barbershop_id, created_at);

CREATE TRIGGER trg_orders_updated
BEFORE UPDATE ON orders
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- ORDER ITEMS
-- ============================================================

CREATE TABLE order_items (
  id                   BIGSERIAL    PRIMARY KEY,
  order_id             BIGINT       NOT NULL REFERENCES orders(id)   ON DELETE CASCADE,
  product_id           BIGINT       NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
  product_name_snapshot VARCHAR(150) NOT NULL,
  quantity             INTEGER      NOT NULL CHECK (quantity > 0),
  unit_price           BIGINT       NOT NULL CHECK (unit_price >= 0),
  line_total           BIGINT       NOT NULL CHECK (line_total >= 0)
);

COMMENT ON COLUMN order_items.unit_price IS 'Item unit price stored in CENTS (int64).';
COMMENT ON COLUMN order_items.line_total IS 'Item line total stored in CENTS (int64).';

CREATE INDEX idx_order_items_order   ON order_items(order_id);
CREATE INDEX idx_order_items_product ON order_items(product_id);

-- ============================================================
-- PAYMENTS
-- ============================================================

CREATE TABLE payments (
  id              BIGSERIAL     PRIMARY KEY,
  barbershop_id   BIGINT        NOT NULL REFERENCES barbershops(id)   ON DELETE CASCADE,
  appointment_id  BIGINT        REFERENCES appointments(id)           ON DELETE CASCADE,
  order_id        BIGINT        REFERENCES orders(id)                 ON DELETE CASCADE,
  bundled_order_id BIGINT       REFERENCES orders(id)                 ON DELETE SET NULL,
  subscription_id BIGINT        REFERENCES subscriptions(id)          ON DELETE SET NULL,
  txid            VARCHAR(100)  UNIQUE,
  qr_code         TEXT,
  amount          BIGINT        NOT NULL CHECK (amount >= 0),
  status          payment_status NOT NULL,
  paid_at         TIMESTAMPTZ,
  expires_at      TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now(),

  CONSTRAINT payment_exactly_one_target CHECK (
    (appointment_id IS NOT NULL AND order_id IS NULL          AND subscription_id IS NULL)
    OR (appointment_id IS NULL  AND order_id IS NOT NULL      AND subscription_id IS NULL)
    OR (appointment_id IS NULL  AND order_id IS NULL          AND subscription_id IS NOT NULL)
  )
);

COMMENT ON COLUMN payments.amount IS 'Payment amount stored in CENTS (int64).';

CREATE INDEX idx_payments_barbershop_status ON payments(barbershop_id, status);
CREATE INDEX idx_payments_appointment       ON payments(barbershop_id, appointment_id);
CREATE INDEX idx_payments_subscription      ON payments(barbershop_id, subscription_id)
  WHERE subscription_id IS NOT NULL;

CREATE UNIQUE INDEX uq_payments_shop_appointment
  ON payments(barbershop_id, appointment_id)
  WHERE appointment_id IS NOT NULL;

CREATE UNIQUE INDEX uq_payments_shop_order
  ON payments(barbershop_id, order_id)
  WHERE order_id IS NOT NULL;

CREATE TRIGGER trg_payments_updated
BEFORE UPDATE ON payments
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- PIX EVENTS
-- ============================================================

CREATE TABLE pix_events (
  id         BIGSERIAL   PRIMARY KEY,
  tx_id      VARCHAR(100) NOT NULL,
  event_type VARCHAR(50)  NOT NULL,
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE UNIQUE INDEX uniq_pix_event ON pix_events(tx_id, event_type);

-- ============================================================
-- CATEGORY PAYMENT POLICIES
-- ============================================================

CREATE TABLE category_payment_policies (
  id            BIGSERIAL          PRIMARY KEY,
  barbershop_id BIGINT             NOT NULL REFERENCES barbershops(id) ON DELETE CASCADE,
  category      client_category    NOT NULL,
  requirement   payment_requirement NOT NULL,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE UNIQUE INDEX uq_category_payment_policy ON category_payment_policies(barbershop_id, category);

CREATE TRIGGER trg_category_payment_policies_updated
BEFORE UPDATE ON category_payment_policies
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- WORKING HOURS
-- ============================================================

CREATE TABLE working_hours (
  id            BIGSERIAL  PRIMARY KEY,
  barber_id     BIGINT     NOT NULL REFERENCES users(id)       ON DELETE CASCADE,
  barbershop_id BIGINT     NOT NULL REFERENCES barbershops(id) ON DELETE CASCADE,
  weekday       INTEGER    NOT NULL,
  start_time    VARCHAR(10),
  end_time      VARCHAR(10),
  lunch_start   VARCHAR(10),
  lunch_end     VARCHAR(10),
  active        BOOLEAN    DEFAULT true,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TRIGGER trg_working_hours_updated
BEFORE UPDATE ON working_hours
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- SCHEDULE OVERRIDES (004)
-- ============================================================

CREATE TABLE schedule_overrides (
  id            BIGSERIAL   PRIMARY KEY,
  barbershop_id BIGINT      NOT NULL REFERENCES barbershops(id) ON DELETE CASCADE,
  barber_id     BIGINT      NOT NULL REFERENCES users(id)       ON DELETE CASCADE,

  -- scope: date XOR (weekday + month + year)
  date          DATE,
  weekday       SMALLINT,
  month         SMALLINT,
  year          SMALLINT,

  closed        BOOLEAN     NOT NULL DEFAULT false,
  start_time    VARCHAR(5),
  end_time      VARCHAR(5),

  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

  CONSTRAINT scope_check CHECK (
    (date IS NOT NULL AND weekday IS NULL AND month IS NULL AND year IS NULL)
    OR
    (date IS NULL AND weekday IS NOT NULL AND month IS NOT NULL AND year IS NOT NULL)
  )
);

CREATE UNIQUE INDEX schedule_overrides_date_idx
  ON schedule_overrides(barber_id, date)
  WHERE date IS NOT NULL;

CREATE UNIQUE INDEX schedule_overrides_weekday_idx
  ON schedule_overrides(barber_id, weekday, month, year)
  WHERE weekday IS NOT NULL;

-- ============================================================
-- IDEMPOTENCY KEYS
-- ============================================================

CREATE TABLE idempotency_keys (
  key        VARCHAR(128) PRIMARY KEY,
  created_at TIMESTAMPTZ  DEFAULT now()
);

-- ============================================================
-- JOB LOCKS
-- ============================================================

CREATE TABLE job_locks (
  job_name     VARCHAR(80)  PRIMARY KEY,
  locked_until TIMESTAMPTZ  NOT NULL,
  locked_by    VARCHAR(128) NOT NULL,
  updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

COMMENT ON TABLE job_locks IS
'Leader-lock por job. Evita rodar jobs em duplicidade quando há múltiplas instâncias.';

CREATE TRIGGER trg_job_locks_updated
BEFORE UPDATE ON job_locks
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- AUDIT LOGS
-- ============================================================

CREATE TABLE audit_logs (
  id            BIGSERIAL PRIMARY KEY,
  barbershop_id BIGINT REFERENCES barbershops(id) ON DELETE SET NULL,
  user_id       BIGINT REFERENCES users(id)       ON DELETE SET NULL,
  action        VARCHAR(50) NOT NULL,
  entity        VARCHAR(50),
  entity_id     BIGINT,
  metadata      TEXT,
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_audit_logs_barbershop_created ON audit_logs(barbershop_id, created_at DESC);

-- ============================================================
-- APPOINTMENT CLOSURES
-- ============================================================

CREATE TABLE appointment_closures (
  id             BIGSERIAL PRIMARY KEY,
  appointment_id BIGINT    NOT NULL UNIQUE REFERENCES appointments(id) ON DELETE CASCADE,
  barbershop_id  BIGINT    NOT NULL REFERENCES barbershops(id) ON DELETE CASCADE,

  service_id             BIGINT  REFERENCES barbershop_services(id) ON DELETE SET NULL,
  service_name           VARCHAR(150),
  reference_amount_cents BIGINT  NOT NULL DEFAULT 0 CHECK (reference_amount_cents >= 0),
  final_amount_cents     BIGINT  CHECK (final_amount_cents >= 0),

  subscription_consume_status VARCHAR(50),
  subscription_plan_id  BIGINT  REFERENCES plans(id) ON DELETE SET NULL,
  subscription_covered  BOOLEAN NOT NULL DEFAULT false,
  requires_normal_charging BOOLEAN NOT NULL DEFAULT true,
  confirm_normal_charging  BOOLEAN NOT NULL DEFAULT false,

  operational_note    VARCHAR(255),
  actual_service_id   BIGINT  REFERENCES barbershop_services(id) ON DELETE SET NULL,
  actual_service_name VARCHAR(150),
  payment_method      VARCHAR(20),
  additional_order_id BIGINT  REFERENCES orders(id) ON DELETE SET NULL,
  suggestion_removed  BOOLEAN NOT NULL DEFAULT false,

  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_appointment_closures_barbershop      ON appointment_closures(barbershop_id);
CREATE INDEX idx_appointment_closures_barbershop_appt ON appointment_closures(barbershop_id, appointment_id);

CREATE TRIGGER trg_appointment_closures_updated
BEFORE UPDATE ON appointment_closures
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- CLOSURE ADJUSTMENTS
-- ============================================================

CREATE TABLE closure_adjustments (
  id            BIGSERIAL PRIMARY KEY,
  closure_id    BIGINT    NOT NULL REFERENCES appointment_closures(id) ON DELETE CASCADE,
  barbershop_id BIGINT    NOT NULL REFERENCES barbershops(id) ON DELETE CASCADE,
  barber_id     BIGINT    REFERENCES users(id) ON DELETE SET NULL,

  delta_final_amount_cents BIGINT  CHECK (delta_final_amount_cents >= 0),
  delta_payment_method     VARCHAR(20),
  delta_operational_note   VARCHAR(255),

  reason      VARCHAR(255) NOT NULL,
  adjusted_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
  created_at  TIMESTAMPTZ  DEFAULT now()
);

COMMENT ON TABLE closure_adjustments IS
'Post-closure corrections within the 7-day window. Never modifies the original closure.';

CREATE INDEX idx_closure_adjustments_closure    ON closure_adjustments(closure_id);
CREATE INDEX idx_closure_adjustments_barbershop ON closure_adjustments(barbershop_id);

-- ============================================================
-- APPOINTMENT TICKETS
-- ============================================================

CREATE TABLE appointment_tickets (
  id             BIGSERIAL   PRIMARY KEY,
  appointment_id BIGINT      NOT NULL UNIQUE REFERENCES appointments(id) ON DELETE CASCADE,
  barbershop_id  BIGINT      NOT NULL REFERENCES barbershops(id) ON DELETE CASCADE,
  token          VARCHAR(64) NOT NULL UNIQUE,
  expires_at     TIMESTAMPTZ NOT NULL,
  created_at     TIMESTAMPTZ DEFAULT now()
);

COMMENT ON TABLE appointment_tickets IS
'Public self-service tokens for clients to view, cancel or reschedule their own appointments.';

CREATE INDEX idx_appointment_tickets_token      ON appointment_tickets(token);
CREATE INDEX idx_appointment_tickets_barbershop ON appointment_tickets(barbershop_id);

-- ============================================================
-- CARTS
-- ============================================================

CREATE TABLE carts (
  key           VARCHAR(128) NOT NULL,
  barbershop_id BIGINT       NOT NULL REFERENCES barbershops(id) ON DELETE CASCADE,
  items         JSONB        NOT NULL DEFAULT '[]',
  expires_at    TIMESTAMPTZ  NOT NULL DEFAULT now() + INTERVAL '24 hours',
  created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),

  PRIMARY KEY (key, barbershop_id)
);

CREATE INDEX idx_carts_barbershop ON carts(barbershop_id);
CREATE INDEX idx_carts_expires_at ON carts(expires_at);

CREATE TRIGGER trg_carts_updated
BEFORE UPDATE ON carts
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;
