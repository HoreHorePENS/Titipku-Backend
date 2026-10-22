CREATE OR REPLACE FUNCTION record_payment(
    p_business_id UUID,
    p_member_id UUID,
    p_payload JSONB,
    p_request_hash TEXT DEFAULT ''
)
RETURNS JSONB
LANGUAGE plpgsql
AS $$
DECLARE
    v_payment_id UUID;
    v_partner_id UUID;
    v_client_request_id UUID;
    v_amount BIGINT;
    v_method TEXT;
    v_paid_at TIMESTAMPTZ;
    v_receivable BIGINT;
    v_remaining BIGINT;
    v_existing_request_hash TEXT;
    v_is_void BOOLEAN;
    v_invoice RECORD;
    v_take BIGINT;
BEGIN
    IF p_payload IS NULL OR jsonb_typeof(p_payload) <> 'object' THEN
        RAISE EXCEPTION 'INVALID_PAYMENT_PAYLOAD';
    END IF;

    v_client_request_id := (p_payload->>'client_request_id')::UUID;
    IF v_client_request_id IS NULL THEN
        RAISE EXCEPTION 'PAYMENT_CLIENT_REQUEST_ID_REQUIRED';
    END IF;

    SELECT id, request_hash, is_void
    INTO v_payment_id, v_existing_request_hash, v_is_void
    FROM payments
    WHERE business_id = p_business_id
      AND client_request_id = v_client_request_id;

    IF v_payment_id IS NOT NULL THEN
        IF p_request_hash <> '' AND v_existing_request_hash <> '' AND p_request_hash <> v_existing_request_hash THEN
            RAISE EXCEPTION 'IDEMPOTENCY_MISMATCH';
        END IF;

        RETURN jsonb_build_object(
            'payment_id', v_payment_id,
            'duplicate', TRUE,
            'is_void', v_is_void
        );
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM members
        WHERE id = p_member_id
          AND business_id = p_business_id
          AND is_active
    ) THEN
        RAISE EXCEPTION 'MEMBER_NOT_FOUND';
    END IF;

    v_partner_id := (p_payload->>'partner_id')::UUID;
    IF v_partner_id IS NULL THEN
        RAISE EXCEPTION 'PARTNER_REQUIRED';
    END IF;

    SELECT id
    INTO v_partner_id
    FROM partners
    WHERE id = v_partner_id
      AND business_id = p_business_id
      AND is_active
    FOR UPDATE;

    IF v_partner_id IS NULL THEN
        RAISE EXCEPTION 'PARTNER_NOT_FOUND';
    END IF;

    v_amount := (p_payload->>'amount')::BIGINT;
    IF v_amount IS NULL OR v_amount <= 0 THEN
        RAISE EXCEPTION 'PAYMENT_AMOUNT_INVALID';
    END IF;

    v_method := COALESCE(NULLIF(btrim(p_payload->>'method'), ''), 'cash');
    IF v_method NOT IN ('cash', 'transfer', 'qris', 'other') THEN
        RAISE EXCEPTION 'PAYMENT_METHOD_INVALID';
    END IF;

    v_paid_at := COALESCE((p_payload->>'paid_at')::TIMESTAMPTZ, now());

    SELECT COALESCE(SUM(total - paid), 0)
    INTO v_receivable
    FROM invoices
    WHERE business_id = p_business_id
      AND partner_id = v_partner_id
      AND paid < total;

    IF v_amount > v_receivable THEN
        RAISE EXCEPTION 'PAYMENT_EXCEEDS_RECEIVABLE';
    END IF;

    v_payment_id := COALESCE((p_payload->>'payment_id')::UUID, gen_random_uuid());

    INSERT INTO payments (
        id, business_id, partner_id, member_id, amount, method, paid_at,
        client_request_id, request_hash
    )
    VALUES (
        v_payment_id, p_business_id, v_partner_id, p_member_id, v_amount, v_method, v_paid_at,
        v_client_request_id, p_request_hash
    );

    v_remaining := v_amount;

    FOR v_invoice IN
        SELECT id, total, paid
        FROM invoices
        WHERE business_id = p_business_id
          AND partner_id = v_partner_id
          AND paid < total
        ORDER BY issued_at, id
        FOR UPDATE
    LOOP
        EXIT WHEN v_remaining <= 0;

        v_take := LEAST(v_invoice.total - v_invoice.paid, v_remaining);

        UPDATE invoices
        SET paid = paid + v_take
        WHERE id = v_invoice.id;

        INSERT INTO payment_allocations (id, payment_id, invoice_id, amount)
        VALUES (gen_random_uuid(), v_payment_id, v_invoice.id, v_take);

        v_remaining := v_remaining - v_take;
    END LOOP;

    IF v_remaining <> 0 THEN
        RAISE EXCEPTION 'PAYMENT_ALLOCATION_FAILED';
    END IF;

    RETURN jsonb_build_object(
        'payment_id', v_payment_id,
        'amount', v_amount,
        'duplicate', FALSE,
        'is_void', FALSE
    );
END;
$$;

CREATE OR REPLACE FUNCTION void_payment(
    p_business_id UUID,
    p_member_id UUID,
    p_payment_id UUID
)
RETURNS JSONB
LANGUAGE plpgsql
AS $$
DECLARE
    v_is_void BOOLEAN;
    v_allocation RECORD;
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM members
        WHERE id = p_member_id
          AND business_id = p_business_id
          AND is_active
    ) THEN
        RAISE EXCEPTION 'MEMBER_NOT_FOUND';
    END IF;

    SELECT is_void
    INTO v_is_void
    FROM payments
    WHERE id = p_payment_id
      AND business_id = p_business_id
    FOR UPDATE;

    IF v_is_void IS NULL THEN
        RAISE EXCEPTION 'PAYMENT_NOT_FOUND';
    END IF;

    IF v_is_void THEN
        RETURN jsonb_build_object('payment_id', p_payment_id, 'is_void', TRUE, 'duplicate', TRUE);
    END IF;

    FOR v_allocation IN
        SELECT invoice_id, amount
        FROM payment_allocations
        WHERE payment_id = p_payment_id
    LOOP
        UPDATE invoices
        SET paid = paid - v_allocation.amount
        WHERE id = v_allocation.invoice_id
          AND paid >= v_allocation.amount;

        IF NOT FOUND THEN
            RAISE EXCEPTION 'PAYMENT_ALLOCATION_INCONSISTENT';
        END IF;
    END LOOP;

    UPDATE payments
    SET is_void = TRUE,
        voided_at = now(),
        voided_by = p_member_id
    WHERE id = p_payment_id;

    RETURN jsonb_build_object('payment_id', p_payment_id, 'is_void', TRUE, 'duplicate', FALSE);
END;
$$;

CREATE OR REPLACE FUNCTION record_visit(
    p_business_id UUID,
    p_member_id UUID,
    p_payload JSONB,
    p_request_hash TEXT DEFAULT ''
)
RETURNS JSONB
LANGUAGE plpgsql
AS $$
DECLARE
    v_visit_id UUID;
    v_partner_id UUID;
    v_partner_owner TEXT;
    v_client_request_id UUID;
    v_public_token TEXT;
    v_visited_at TIMESTAMPTZ;
    v_received_by TEXT;
    v_sequence BIGINT;
    v_number TEXT;
    v_items JSONB;
    v_item JSONB;
    v_product_id UUID;
    v_product_name TEXT;
    v_unit_price BIGINT;
    v_shelf_life_days INTEGER;
    v_remaining BIGINT;
    v_returned BIGINT;
    v_new_drop BIGINT;
    v_prev_stock BIGINT;
    v_sold BIGINT;
    v_stock_after BIGINT;
    v_deduct BIGINT;
    v_batch RECORD;
    v_take BIGINT;
    v_total BIGINT := 0;
    v_invoice_id UUID;
    v_existing_visit_id UUID;
    v_existing_request_hash TEXT;
    v_payment JSONB := NULL;
    v_payment_result JSONB := NULL;
    v_dropped_at DATE;
BEGIN
    IF p_payload IS NULL OR jsonb_typeof(p_payload) <> 'object' THEN
        RAISE EXCEPTION 'INVALID_VISIT_PAYLOAD';
    END IF;

    v_client_request_id := (p_payload->>'client_request_id')::UUID;
    IF v_client_request_id IS NULL THEN
        RAISE EXCEPTION 'VISIT_CLIENT_REQUEST_ID_REQUIRED';
    END IF;

    SELECT id, request_hash
    INTO v_existing_visit_id, v_existing_request_hash
    FROM visits
    WHERE business_id = p_business_id
      AND client_request_id = v_client_request_id;

    IF v_existing_visit_id IS NOT NULL THEN
        IF p_request_hash <> '' AND v_existing_request_hash <> '' AND p_request_hash <> v_existing_request_hash THEN
            RAISE EXCEPTION 'IDEMPOTENCY_MISMATCH';
        END IF;

        SELECT id
        INTO v_invoice_id
        FROM invoices
        WHERE visit_id = v_existing_visit_id;

        RETURN jsonb_build_object(
            'visit_id', v_existing_visit_id,
            'invoice_id', v_invoice_id,
            'duplicate', TRUE
        );
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM members
        WHERE id = p_member_id
          AND business_id = p_business_id
          AND is_active
    ) THEN
        RAISE EXCEPTION 'MEMBER_NOT_FOUND';
    END IF;

    SELECT visit_sequence
    INTO v_sequence
    FROM businesses
    WHERE id = p_business_id
    FOR UPDATE;

    IF v_sequence IS NULL THEN
        RAISE EXCEPTION 'BUSINESS_NOT_FOUND';
    END IF;

    v_partner_id := (p_payload->>'partner_id')::UUID;
    IF v_partner_id IS NULL THEN
        RAISE EXCEPTION 'PARTNER_REQUIRED';
    END IF;

    SELECT owner_name
    INTO v_partner_owner
    FROM partners
    WHERE id = v_partner_id
      AND business_id = p_business_id
      AND is_active
    FOR UPDATE;

    IF v_partner_owner IS NULL THEN
        RAISE EXCEPTION 'PARTNER_NOT_FOUND';
    END IF;

    v_items := p_payload->'items';
    IF v_items IS NULL OR jsonb_typeof(v_items) <> 'array' OR jsonb_array_length(v_items) = 0 THEN
        RAISE EXCEPTION 'VISIT_ITEMS_REQUIRED';
    END IF;

    IF (
        SELECT count(*)
        FROM jsonb_array_elements(v_items)
    ) <> (
        SELECT count(DISTINCT value->>'product_id')
        FROM jsonb_array_elements(v_items)
    ) THEN
        RAISE EXCEPTION 'DUPLICATE_VISIT_PRODUCT';
    END IF;

    v_visited_at := COALESCE((p_payload->>'visited_at')::TIMESTAMPTZ, now());
    v_received_by := COALESCE(NULLIF(btrim(p_payload->>'received_by'), ''), v_partner_owner);
    v_public_token := COALESCE(NULLIF(p_payload->>'public_token', ''), encode(gen_random_bytes(32), 'base64url'));
    v_visit_id := COALESCE((p_payload->>'visit_id')::UUID, gen_random_uuid());

    UPDATE businesses
    SET visit_sequence = visit_sequence + 1
    WHERE id = p_business_id
    RETURNING visit_sequence INTO v_sequence;

    v_number := 'N-' || to_char(v_visited_at AT TIME ZONE 'Asia/Jakarta', 'YYMMDD')
        || '-' || lpad(v_sequence::TEXT, 4, '0');

    INSERT INTO visits (
        id, business_id, partner_id, member_id, number, client_request_id,
        request_hash, public_token, visited_at, received_by
    )
    VALUES (
        v_visit_id, p_business_id, v_partner_id, p_member_id, v_number, v_client_request_id,
        p_request_hash, v_public_token, v_visited_at, v_received_by
    );

    FOR v_item IN SELECT value FROM jsonb_array_elements(v_items)
    LOOP
        v_product_id := (v_item->>'product_id')::UUID;
        IF v_product_id IS NULL THEN
            RAISE EXCEPTION 'PRODUCT_REQUIRED';
        END IF;

        SELECT name, consign_price, shelf_life_days
        INTO v_product_name, v_unit_price, v_shelf_life_days
        FROM products
        WHERE id = v_product_id
          AND business_id = p_business_id
          AND is_active
        FOR SHARE;

        IF v_product_name IS NULL THEN
            RAISE EXCEPTION 'PRODUCT_NOT_FOUND';
        END IF;

        v_remaining := COALESCE((v_item->>'remaining')::BIGINT, 0);
        v_returned := COALESCE((v_item->>'returned')::BIGINT, 0);
        v_new_drop := COALESCE((v_item->>'new_drop')::BIGINT, 0);

        IF v_remaining < 0 OR v_returned < 0 OR v_new_drop < 0 THEN
            RAISE EXCEPTION 'VISIT_QUANTITY_INVALID';
        END IF;

        SELECT COALESCE(SUM(qty_left), 0)
        INTO v_prev_stock
        FROM batches
        WHERE business_id = p_business_id
          AND partner_id = v_partner_id
          AND product_id = v_product_id;

        v_sold := v_prev_stock - v_remaining - v_returned;
        IF v_sold < 0 THEN
            RAISE EXCEPTION 'STOCK_MISMATCH';
        END IF;

        v_stock_after := v_remaining + v_new_drop;
        v_deduct := v_sold + v_returned;

        FOR v_batch IN
            SELECT id, qty_left
            FROM batches
            WHERE business_id = p_business_id
              AND partner_id = v_partner_id
              AND product_id = v_product_id
              AND qty_left > 0
            ORDER BY dropped_at, created_at, id
            FOR UPDATE
        LOOP
            EXIT WHEN v_deduct <= 0;

            v_take := LEAST(v_batch.qty_left, v_deduct);

            UPDATE batches
            SET qty_left = qty_left - v_take
            WHERE id = v_batch.id;

            v_deduct := v_deduct - v_take;
        END LOOP;

        IF v_deduct <> 0 THEN
            RAISE EXCEPTION 'STOCK_MISMATCH';
        END IF;

        IF v_new_drop > 0 THEN
            v_dropped_at := (v_visited_at AT TIME ZONE 'Asia/Jakarta')::DATE;

            INSERT INTO batches (
                id, business_id, partner_id, product_id, visit_id,
                dropped_at, expires_at, qty_in, qty_left
            )
            VALUES (
                gen_random_uuid(), p_business_id, v_partner_id, v_product_id, v_visit_id,
                v_dropped_at, v_dropped_at + v_shelf_life_days, v_new_drop, v_new_drop
            );
        END IF;

        IF (
            SELECT COALESCE(SUM(qty_left), 0)
            FROM batches
            WHERE business_id = p_business_id
              AND partner_id = v_partner_id
              AND product_id = v_product_id
        ) <> v_stock_after THEN
            RAISE EXCEPTION 'STOCK_MISMATCH';
        END IF;

        INSERT INTO visit_items (
            id, business_id, visit_id, product_id, product_name, prev_stock,
            remaining, returned, new_drop, sold, stock_after, unit_price, subtotal
        )
        VALUES (
            gen_random_uuid(), p_business_id, v_visit_id, v_product_id, v_product_name, v_prev_stock,
            v_remaining, v_returned, v_new_drop, v_sold, v_stock_after, v_unit_price,
            v_sold * v_unit_price
        );

        v_total := v_total + (v_sold * v_unit_price);
    END LOOP;

    IF v_total > 0 THEN
        v_invoice_id := COALESCE((p_payload->>'invoice_id')::UUID, gen_random_uuid());

        INSERT INTO invoices (id, business_id, partner_id, visit_id, total)
        VALUES (v_invoice_id, p_business_id, v_partner_id, v_visit_id, v_total);
    END IF;

    IF p_payload ? 'payment' AND jsonb_typeof(p_payload->'payment') = 'object' THEN
        v_payment := p_payload->'payment';

        IF COALESCE((v_payment->>'amount')::BIGINT, 0) > 0 THEN
            v_payment_result := record_payment(
                p_business_id,
                p_member_id,
                jsonb_build_object(
                    'payment_id', v_payment->>'payment_id',
                    'partner_id', v_partner_id,
                    'amount', v_payment->>'amount',
                    'method', v_payment->>'method',
                    'paid_at', v_payment->>'paid_at',
                    'client_request_id', v_payment->>'client_request_id'
                ),
                COALESCE(v_payment->>'request_hash', '')
            );
        END IF;
    END IF;

    RETURN jsonb_build_object(
        'visit_id', v_visit_id,
        'invoice_id', v_invoice_id,
        'number', v_number,
        'public_token', v_public_token,
        'total', v_total,
        'duplicate', FALSE,
        'payment', v_payment_result
    );
END;
$$;
