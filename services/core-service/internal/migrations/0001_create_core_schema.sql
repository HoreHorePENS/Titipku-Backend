CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    email TEXT NOT NULL,
    full_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'owner' CHECK (role IN ('owner', 'admin')),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS users_email_unique
    ON users (lower(email));

CREATE TABLE IF NOT EXISTS businesses (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL CHECK (char_length(btrim(name)) BETWEEN 2 AND 120),
    owner_name TEXT NOT NULL CHECK (char_length(btrim(owner_name)) BETWEEN 2 AND 100),
    whatsapp TEXT,
    address TEXT,
    visit_sequence BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS members (
    id UUID PRIMARY KEY,
    business_id UUID NOT NULL REFERENCES businesses (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'owner' CHECK (role IN ('owner', 'staff')),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (business_id, user_id)
);

CREATE INDEX IF NOT EXISTS members_business_id_idx ON members (business_id);
CREATE UNIQUE INDEX IF NOT EXISTS members_active_user_unique
    ON members (user_id)
    WHERE is_active;

CREATE TABLE IF NOT EXISTS products (
    id UUID PRIMARY KEY,
    business_id UUID NOT NULL REFERENCES businesses (id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
    photo_path TEXT,
    consign_price BIGINT NOT NULL CHECK (consign_price >= 0),
    retail_price BIGINT NOT NULL CHECK (retail_price >= 0),
    shelf_life_days INTEGER NOT NULL CHECK (shelf_life_days > 0),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS products_business_id_idx ON products (business_id);
CREATE INDEX IF NOT EXISTS products_business_active_idx ON products (business_id, is_active);

CREATE TABLE IF NOT EXISTS partners (
    id UUID PRIMARY KEY,
    business_id UUID NOT NULL REFERENCES businesses (id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (char_length(btrim(name)) BETWEEN 2 AND 120),
    owner_name TEXT NOT NULL CHECK (char_length(btrim(owner_name)) BETWEEN 2 AND 100),
    whatsapp TEXT NOT NULL CHECK (whatsapp ~ '^62[0-9]{8,15}$'),
    address TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (business_id, whatsapp)
);

CREATE INDEX IF NOT EXISTS partners_business_id_idx ON partners (business_id);
CREATE INDEX IF NOT EXISTS partners_business_active_idx ON partners (business_id, is_active);

CREATE TABLE IF NOT EXISTS visits (
    id UUID PRIMARY KEY,
    business_id UUID NOT NULL REFERENCES businesses (id) ON DELETE CASCADE,
    partner_id UUID NOT NULL REFERENCES partners (id) ON DELETE RESTRICT,
    member_id UUID NOT NULL REFERENCES members (id) ON DELETE RESTRICT,
    number TEXT NOT NULL,
    client_request_id UUID NOT NULL,
    request_hash TEXT NOT NULL DEFAULT '',
    public_token TEXT NOT NULL,
    visited_at TIMESTAMPTZ NOT NULL,
    received_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (business_id, number),
    UNIQUE (business_id, client_request_id),
    UNIQUE (public_token)
);

CREATE INDEX IF NOT EXISTS visits_business_visited_idx
    ON visits (business_id, visited_at DESC);
CREATE INDEX IF NOT EXISTS visits_partner_visited_idx
    ON visits (partner_id, visited_at DESC);

CREATE TABLE IF NOT EXISTS visit_items (
    id UUID PRIMARY KEY,
    business_id UUID NOT NULL REFERENCES businesses (id) ON DELETE CASCADE,
    visit_id UUID NOT NULL REFERENCES visits (id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    product_name TEXT NOT NULL,
    prev_stock BIGINT NOT NULL CHECK (prev_stock >= 0),
    remaining BIGINT NOT NULL CHECK (remaining >= 0),
    returned BIGINT NOT NULL DEFAULT 0 CHECK (returned >= 0),
    new_drop BIGINT NOT NULL DEFAULT 0 CHECK (new_drop >= 0),
    sold BIGINT NOT NULL CHECK (sold >= 0),
    stock_after BIGINT NOT NULL CHECK (stock_after >= 0),
    unit_price BIGINT NOT NULL CHECK (unit_price >= 0),
    subtotal BIGINT NOT NULL CHECK (subtotal >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (visit_id, product_id)
);

CREATE INDEX IF NOT EXISTS visit_items_visit_id_idx ON visit_items (visit_id);
CREATE INDEX IF NOT EXISTS visit_items_business_product_idx ON visit_items (business_id, product_id);

CREATE TABLE IF NOT EXISTS batches (
    id UUID PRIMARY KEY,
    business_id UUID NOT NULL REFERENCES businesses (id) ON DELETE CASCADE,
    partner_id UUID NOT NULL REFERENCES partners (id) ON DELETE RESTRICT,
    product_id UUID NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    visit_id UUID NOT NULL REFERENCES visits (id) ON DELETE CASCADE,
    dropped_at DATE NOT NULL,
    expires_at DATE NOT NULL,
    qty_in BIGINT NOT NULL CHECK (qty_in > 0),
    qty_left BIGINT NOT NULL CHECK (qty_left >= 0 AND qty_left <= qty_in),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS batches_fifo_idx
    ON batches (partner_id, product_id, dropped_at, created_at, id);
CREATE INDEX IF NOT EXISTS batches_business_idx ON batches (business_id);

CREATE TABLE IF NOT EXISTS invoices (
    id UUID PRIMARY KEY,
    business_id UUID NOT NULL REFERENCES businesses (id) ON DELETE CASCADE,
    partner_id UUID NOT NULL REFERENCES partners (id) ON DELETE RESTRICT,
    visit_id UUID NOT NULL REFERENCES visits (id) ON DELETE CASCADE,
    total BIGINT NOT NULL CHECK (total > 0),
    paid BIGINT NOT NULL DEFAULT 0 CHECK (paid >= 0 AND paid <= total),
    status TEXT GENERATED ALWAYS AS (
        CASE
            WHEN paid = 0 THEN 'unpaid'
            WHEN paid < total THEN 'partial'
            ELSE 'paid'
        END
    ) STORED,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (visit_id)
);

CREATE INDEX IF NOT EXISTS invoices_business_status_idx
    ON invoices (business_id, status, issued_at);
CREATE INDEX IF NOT EXISTS invoices_partner_issued_idx
    ON invoices (partner_id, issued_at, id);

CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY,
    business_id UUID NOT NULL REFERENCES businesses (id) ON DELETE CASCADE,
    partner_id UUID NOT NULL REFERENCES partners (id) ON DELETE RESTRICT,
    member_id UUID NOT NULL REFERENCES members (id) ON DELETE RESTRICT,
    amount BIGINT NOT NULL CHECK (amount > 0),
    method TEXT NOT NULL CHECK (method IN ('cash', 'transfer', 'qris', 'other')),
    paid_at TIMESTAMPTZ NOT NULL,
    client_request_id UUID NOT NULL,
    request_hash TEXT NOT NULL DEFAULT '',
    is_void BOOLEAN NOT NULL DEFAULT FALSE,
    voided_at TIMESTAMPTZ,
    voided_by UUID REFERENCES members (id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (business_id, client_request_id)
);

CREATE INDEX IF NOT EXISTS payments_business_paid_at_idx
    ON payments (business_id, paid_at DESC);
CREATE INDEX IF NOT EXISTS payments_partner_paid_at_idx
    ON payments (partner_id, paid_at DESC);

CREATE TABLE IF NOT EXISTS payment_allocations (
    id UUID PRIMARY KEY,
    payment_id UUID NOT NULL REFERENCES payments (id) ON DELETE CASCADE,
    invoice_id UUID NOT NULL REFERENCES invoices (id) ON DELETE RESTRICT,
    amount BIGINT NOT NULL CHECK (amount > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (payment_id, invoice_id)
);

CREATE INDEX IF NOT EXISTS payment_allocations_invoice_id_idx
    ON payment_allocations (invoice_id);

CREATE OR REPLACE FUNCTION titipku_set_updated_at()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS businesses_set_updated_at ON businesses;
CREATE TRIGGER businesses_set_updated_at
    BEFORE UPDATE ON businesses
    FOR EACH ROW EXECUTE FUNCTION titipku_set_updated_at();

DROP TRIGGER IF EXISTS members_set_updated_at ON members;
CREATE TRIGGER members_set_updated_at
    BEFORE UPDATE ON members
    FOR EACH ROW EXECUTE FUNCTION titipku_set_updated_at();

DROP TRIGGER IF EXISTS products_set_updated_at ON products;
CREATE TRIGGER products_set_updated_at
    BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION titipku_set_updated_at();

DROP TRIGGER IF EXISTS partners_set_updated_at ON partners;
CREATE TRIGGER partners_set_updated_at
    BEFORE UPDATE ON partners
    FOR EACH ROW EXECUTE FUNCTION titipku_set_updated_at();

DROP TRIGGER IF EXISTS visits_set_updated_at ON visits;
CREATE TRIGGER visits_set_updated_at
    BEFORE UPDATE ON visits
    FOR EACH ROW EXECUTE FUNCTION titipku_set_updated_at();

DROP TRIGGER IF EXISTS invoices_set_updated_at ON invoices;
CREATE TRIGGER invoices_set_updated_at
    BEFORE UPDATE ON invoices
    FOR EACH ROW EXECUTE FUNCTION titipku_set_updated_at();

DROP TRIGGER IF EXISTS payments_set_updated_at ON payments;
CREATE TRIGGER payments_set_updated_at
    BEFORE UPDATE ON payments
    FOR EACH ROW EXECUTE FUNCTION titipku_set_updated_at();
