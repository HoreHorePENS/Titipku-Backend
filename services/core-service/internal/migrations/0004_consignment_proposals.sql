-- Proposal kemitraan konsinyasi antara toko (pemilik_toko) dan bisnis penitip.
-- Dua arah:
--   store_to_penitip  = "tawaran": toko menawarkan rak/tempat ke penitip.
--   penitip_to_store  = "pengajuan": penitip mengajukan titip-jual ke toko.
-- Penerima menyetujui/menolak; pengirim dapat membatalkan selama masih pending.
-- Saat disetujui, toko menjadi partner bisnis penitip (partners.store_id).
CREATE TABLE IF NOT EXISTS consignment_proposals (
    id UUID PRIMARY KEY,
    store_id UUID NOT NULL REFERENCES stores (id) ON DELETE CASCADE,
    business_id UUID NOT NULL REFERENCES businesses (id) ON DELETE CASCADE,
    direction TEXT NOT NULL CHECK (direction IN ('store_to_penitip', 'penitip_to_store')),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'accepted', 'rejected', 'cancelled')),
    message TEXT CHECK (message IS NULL OR char_length(message) <= 1000),
    shelf_location TEXT CHECK (shelf_location IS NULL OR char_length(shelf_location) <= 120),
    estimated_capacity TEXT CHECK (estimated_capacity IS NULL OR char_length(estimated_capacity) <= 60),
    created_by_user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    responded_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

-- Hanya satu proposal pending per pasangan toko-bisnis (arah mana pun).
CREATE UNIQUE INDEX IF NOT EXISTS consignment_proposals_pending_unique
    ON consignment_proposals (store_id, business_id)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS consignment_proposals_store_idx ON consignment_proposals (store_id, created_at DESC);
CREATE INDEX IF NOT EXISTS consignment_proposals_business_idx ON consignment_proposals (business_id, created_at DESC);

DROP TRIGGER IF EXISTS consignment_proposals_set_updated_at ON consignment_proposals;
CREATE TRIGGER consignment_proposals_set_updated_at
    BEFORE UPDATE ON consignment_proposals
    FOR EACH ROW EXECUTE FUNCTION titipku_set_updated_at();

-- Rencana produk yang akan dititipkan (pengajuan penitip). Informasi saja:
-- stok sebenarnya tercatat lewat kunjungan setelah kemitraan disetujui.
CREATE TABLE IF NOT EXISTS consignment_proposal_items (
    proposal_id UUID NOT NULL REFERENCES consignment_proposals (id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    qty INTEGER NOT NULL CHECK (qty BETWEEN 1 AND 100000),
    PRIMARY KEY (proposal_id, product_id)
);
