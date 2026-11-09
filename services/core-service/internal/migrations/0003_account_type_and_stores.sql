-- Dua persona akun: penitip (produsen) dan pemilik_toko.
-- Default 'penitip' agar pengguna lama tetap mendapat pengalaman yang sama.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS account_type TEXT NOT NULL DEFAULT 'penitip'
    CONSTRAINT users_account_type_check CHECK (account_type IN ('penitip', 'pemilik_toko'));

-- Katalog toko global: terlihat oleh semua penitip yang login.
CREATE TABLE IF NOT EXISTS stores (
    id UUID PRIMARY KEY,
    owner_user_id UUID NOT NULL UNIQUE REFERENCES users (id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (char_length(btrim(name)) BETWEEN 2 AND 120),
    category TEXT NOT NULL DEFAULT 'lainnya'
        CHECK (category IN ('warung', 'kelontong', 'kafe', 'resto', 'minimarket', 'apotek', 'roti', 'lainnya')),
    description TEXT CHECK (description IS NULL OR char_length(btrim(description)) <= 1000),
    address TEXT NOT NULL CHECK (char_length(btrim(address)) BETWEEN 3 AND 300),
    whatsapp TEXT NOT NULL CHECK (whatsapp ~ '^62[0-9]{8,15}$'),
    latitude DOUBLE PRECISION NOT NULL CHECK (latitude BETWEEN -90 AND 90),
    longitude DOUBLE PRECISION NOT NULL CHECK (longitude BETWEEN -180 AND 180),
    photo_url TEXT CHECK (photo_url IS NULL OR char_length(photo_url) <= 500),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS stores_active_category_idx ON stores (is_active, category);
CREATE INDEX IF NOT EXISTS stores_coords_idx ON stores (latitude, longitude);

DROP TRIGGER IF EXISTS stores_set_updated_at ON stores;
CREATE TRIGGER stores_set_updated_at
    BEFORE UPDATE ON stores
    FOR EACH ROW EXECUTE FUNCTION titipku_set_updated_at();

-- Jembatan: partner milik penitip yang berasal dari toko di katalog.
ALTER TABLE partners
    ADD COLUMN IF NOT EXISTS store_id UUID REFERENCES stores (id);

-- Satu partner per toko per bisnis (mencegah titipan dobel dari penitip yang sama).
CREATE UNIQUE INDEX IF NOT EXISTS partners_store_unique
    ON partners (business_id, store_id)
    WHERE store_id IS NOT NULL;
