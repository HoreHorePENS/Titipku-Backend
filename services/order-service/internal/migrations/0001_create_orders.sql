CREATE TABLE IF NOT EXISTS orders (
    id UUID PRIMARY KEY,
    customer_id UUID NOT NULL,
    courier_id UUID,
    item_name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    pickup_address TEXT NOT NULL,
    dropoff_address TEXT NOT NULL,
    item_price BIGINT NOT NULL CHECK (item_price >= 0),
    service_fee BIGINT NOT NULL DEFAULT 0 CHECK (service_fee >= 0),
    status TEXT NOT NULL CHECK (status IN ('pending', 'accepted', 'picked_up', 'delivered', 'cancelled')),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS orders_customer_created_idx
    ON orders (customer_id, created_at DESC);

CREATE INDEX IF NOT EXISTS orders_courier_created_idx
    ON orders (courier_id, created_at DESC);

CREATE INDEX IF NOT EXISTS orders_status_idx
    ON orders (status);
