-- KD sample RMAs (fictional data only) for demos: one in every step, across
-- Dahua, Ajax and Alta Labs. Most sit on the sample tickets; the rest get a
-- ticket of their own, as RMAs from the form do. Each has its history and
-- the matching lines on the ticket's timeline.
-- Re-runnable: replaces earlier sample RMAs and what was made for them.
-- Needs seed.sql and seed-customers.sql first, and the RMA tables (the app
-- creates them on start / --upgrade).
BEGIN;

DELETE FROM kd_rmas WHERE kd_sample;
DELETE FROM conversations WHERE meta->>'kd_sample_rma' = 'true';
DELETE FROM conversation_messages WHERE meta->>'kd_sample_rma' = 'true';
DELETE FROM users WHERE custom_attributes->>'kd_sample_rma' = 'true';

-- An installer who isn't a KD customer yet: shows the "No customer" case
INSERT INTO users (type, email, first_name, last_name, phone_number, custom_attributes)
VALUES ('contact', 'jim.hennessy.installs@gmail.com', 'Jim', 'Hennessy', '0861239876',
        '{"kd_sample": true, "kd_sample_rma": true}');

CREATE FUNCTION pg_temp.kd_sample_rma(
  p_contact TEXT,       -- requester's email (a sample contact)
  p_ticket TEXT,        -- subject of the sample ticket it sits on, or NULL to open one
  p_source TEXT,        -- 'form' or 'agent'
  p_status TEXT,
  p_outcome TEXT,
  p_brand TEXT, p_product TEXT, p_qty INT, p_serials TEXT, p_fault TEXT,
  p_invoice TEXT, p_bought DATE, p_warranty TEXT, p_test TEXT,
  p_supplier_ref TEXT, p_credit TEXT, p_replacement TEXT,
  p_days_ago NUMERIC,   -- when it was requested
  p_agent TEXT,         -- agent who worked it (email); NULL = nobody yet
  p_note TEXT           -- note on its latest step (e.g. why it was rejected)
) RETURNS void LANGUAGE plpgsql AS $fn$
DECLARE
  v_contact BIGINT; v_customer INT; v_agent BIGINT; v_agent_name TEXT;
  v_conv BIGINT; v_id INT; v_no TEXT; v_line TEXT; v_body TEXT; v_text TEXT; v_reply TEXT;
  v_created TIMESTAMPTZ := NOW() - make_interval(secs => (p_days_ago * 86400)::int);
  v_life NUMERIC := p_days_ago * 86400;
  v_closed BOOL := p_status IN ('resolved', 'rejected');
  v_steps TEXT[]; v_step TEXT; v_prev TEXT := 'requested'; v_at TIMESTAMPTZ; v_note TEXT; v_msg TEXT;
  v_inbox INT := (SELECT id FROM inboxes WHERE config->>'kd_sample' = 'true');
  v_team INT := (SELECT id FROM teams WHERE name = 'Support' AND emoji = '🛠');
  v_labels JSONB := '{"approved": "Approved", "received": "Received", "with_supplier": "With supplier", "resolved": "Resolved", "rejected": "Rejected"}';
  v_outcomes JSONB := '{"replaced": "Replaced", "repaired": "Repaired", "credited": "Credited", "returned_nff": "Returned – no fault found"}';
  -- when each step happened, as a share of the RMA's age
  v_offsets JSONB := '{"approved": 0.15, "received": 0.4, "with_supplier": 0.6, "resolved": 0.85, "rejected": 0.25}';
  v_activity JSONB := '{"activity_type": "kd_rma", "kd_sample_rma": true}';
BEGIN
  SELECT id INTO v_contact FROM users WHERE email = p_contact AND type = 'contact';
  SELECT kc.customer_id INTO v_customer FROM kd_contact_customers kc WHERE kc.contact_id = v_contact;
  SELECT id, TRIM(first_name || ' ' || COALESCE(last_name, '')) INTO v_agent, v_agent_name
  FROM users WHERE email = p_agent AND type = 'agent';
  v_line := p_qty || ' × ' || CASE WHEN p_product ILIKE p_brand || '%' THEN p_product ELSE p_brand || ' ' || p_product END;

  INSERT INTO kd_rmas (created_at, updated_at, status, outcome, source, customer_id, contact_id,
    brand, product, quantity, serial_numbers, fault, invoice_number, purchase_date, warranty, test_result,
    supplier, supplier_rma_ref, credit_note_number, replacement_order_number, kd_sample)
  VALUES (v_created, v_created, p_status, COALESCE(p_outcome, ''), p_source, v_customer, v_contact,
    p_brand, p_product, p_qty, p_serials, p_fault, COALESCE(p_invoice, ''), p_bought,
    COALESCE(p_warranty, ''), COALESCE(p_test, ''),
    CASE lower(p_brand) WHEN 'dahua' THEN 'Dahua Technology' WHEN 'ajax' THEN 'Ajax Systems' WHEN 'alta labs' THEN 'Alta Labs' ELSE '' END,
    COALESCE(p_supplier_ref, ''), COALESCE(p_credit, ''), COALESCE(p_replacement, ''), true)
  RETURNING id, number INTO v_id, v_no;

  IF p_ticket IS NOT NULL THEN
    -- On an existing sample ticket, raised by the agent
    SELECT id INTO v_conv FROM conversations WHERE subject = p_ticket AND meta->>'kd_sample' = 'true' LIMIT 1;
    v_msg := v_agent_name || ' raised ' || v_no || ' for ' || v_line;
    INSERT INTO conversation_messages (type, status, private, conversation_id, content_type, content, text_content, sender_id, sender_type, created_at, meta)
    VALUES ('activity', 'sent', true, v_conv, 'text', v_msg, v_msg, v_agent, 'agent', v_created, v_activity);
  ELSE
    -- Its own ticket, as the form (or New RMA) makes
    v_body := '<p>' || CASE WHEN p_source = 'form' THEN 'RMA request sent through the RMA form.' ELSE 'Logged by an agent.' END
      || ' RMA number: <strong>' || v_no || '</strong></p><p><strong>Product:</strong> ' || v_line
      || CASE WHEN p_serials <> '' THEN '<br><strong>Serial numbers:</strong> ' || replace(p_serials, E'\n', ', ') ELSE '' END
      || CASE WHEN COALESCE(p_invoice, '') <> '' THEN '<br><strong>Invoice / order:</strong> ' || p_invoice ELSE '' END
      || CASE WHEN p_bought IS NOT NULL THEN '<br><strong>Purchased:</strong> ' || to_char(p_bought, 'FMDD Mon YYYY') ELSE '' END
      || '</p><p><strong>Fault:</strong><br>' || p_fault || '</p>';
    v_text := trim(regexp_replace(replace(replace(v_body, '<br>', E'\n'), '</p>', E'\n'), '<[^>]+>', '', 'g'));

    INSERT INTO conversations (contact_id, assigned_user_id, assigned_team_id, inbox_id, status_id, priority_id,
                               subject, created_at, updated_at, meta)
    VALUES (v_contact, v_agent, v_team, v_inbox,
            (SELECT id FROM conversation_statuses WHERE name = CASE WHEN v_closed THEN 'Resolved' ELSE 'Open' END),
            (SELECT id FROM conversation_priorities WHERE name = 'Medium'),
            v_no || ': ' || v_line, v_created, v_created, '{"kd_sample": true, "kd_sample_rma": true}')
    RETURNING id INTO v_conv;

    INSERT INTO conversation_messages (type, status, conversation_id, content_type, content, text_content, sender_id, sender_type, created_at)
    VALUES ('incoming', 'received', v_conv, 'html', v_body, v_text, v_contact, 'contact', v_created);
    IF v_agent IS NOT NULL THEN
      v_reply := 'Thanks - we''ve logged ' || v_no || '. Please send the goods to our Cork warehouse with the RMA number on the box. - KD Support';
      INSERT INTO conversation_messages (type, status, conversation_id, content_type, content, text_content, sender_id, sender_type, created_at)
      VALUES ('outgoing', 'sent', v_conv, 'text', v_reply, v_reply, v_agent, 'agent', v_created + interval '40 minutes');
    END IF;

    UPDATE conversations SET
      last_message = COALESCE(v_reply, v_text),
      last_message_at = v_created + CASE WHEN v_agent IS NULL THEN interval '0' ELSE interval '40 minutes' END,
      last_message_sender = CASE WHEN v_agent IS NULL THEN 'contact' ELSE 'agent' END::message_sender_type,
      last_interaction = COALESCE(v_reply, v_text),
      last_interaction_at = v_created + CASE WHEN v_agent IS NULL THEN interval '0' ELSE interval '40 minutes' END,
      last_interaction_sender = CASE WHEN v_agent IS NULL THEN 'contact' ELSE 'agent' END::message_sender_type,
      first_reply_at = CASE WHEN v_agent IS NOT NULL THEN v_created + interval '40 minutes' END,
      last_reply_at = CASE WHEN v_agent IS NOT NULL THEN v_created + interval '40 minutes' END,
      waiting_since = CASE WHEN v_agent IS NULL THEN v_created END,
      resolved_at = CASE WHEN v_closed THEN v_created + make_interval(secs => (v_life * 0.85)::int) END
    WHERE id = v_conv;

    IF v_customer IS NOT NULL THEN
      INSERT INTO kd_conversation_customers (conversation_id, customer_id) VALUES (v_conv, v_customer)
      ON CONFLICT (conversation_id) DO NOTHING;
    END IF;
  END IF;
  UPDATE kd_rmas SET conversation_id = v_conv WHERE id = v_id;

  -- History: created, then each step up to where it is now
  INSERT INTO kd_rma_events (rma_id, created_at, user_id, kind, to_status, note)
  VALUES (v_id, v_created, CASE WHEN p_source = 'agent' THEN v_agent END, 'created', 'requested',
          CASE WHEN p_source = 'agent' THEN 'Raised by an agent'
               WHEN v_customer IS NULL THEN 'From the RMA form - no matching customer, link one'
               ELSE 'From the RMA form' END);

  v_steps := CASE p_status
    WHEN 'approved' THEN ARRAY['approved']
    WHEN 'received' THEN ARRAY['approved', 'received']
    WHEN 'with_supplier' THEN ARRAY['approved', 'received', 'with_supplier']
    WHEN 'resolved' THEN CASE WHEN p_supplier_ref IS NOT NULL
                              THEN ARRAY['approved', 'received', 'with_supplier', 'resolved']
                              ELSE ARRAY['approved', 'received', 'resolved'] END
    WHEN 'rejected' THEN ARRAY['rejected']
    ELSE ARRAY[]::TEXT[] END;

  FOREACH v_step IN ARRAY v_steps LOOP
    v_at := v_created + make_interval(secs => (v_life * (v_offsets->>v_step)::numeric)::int);
    v_note := CASE WHEN v_step <> p_status THEN ''
                   WHEN v_step = 'resolved' THEN trim((v_outcomes->>p_outcome) || '. ' || COALESCE(p_note, ''))
                   ELSE COALESCE(p_note, '') END;
    INSERT INTO kd_rma_events (rma_id, created_at, user_id, kind, from_status, to_status, note)
    VALUES (v_id, v_at, v_agent, 'status', v_prev, v_step, v_note);

    v_msg := v_agent_name || ' moved ' || v_no || ' to ' || (v_labels->>v_step)
      || CASE WHEN v_step = 'resolved' THEN ' (' || (v_outcomes->>p_outcome) || ')' ELSE '' END
      || CASE WHEN v_step = p_status AND COALESCE(p_note, '') <> '' THEN ': ' || p_note ELSE '' END;
    INSERT INTO conversation_messages (type, status, private, conversation_id, content_type, content, text_content, sender_id, sender_type, created_at, meta)
    VALUES ('activity', 'sent', true, v_conv, 'text', v_msg, v_msg, v_agent, 'agent', v_at, v_activity);

    UPDATE kd_rmas SET
      approved_at = CASE WHEN v_step = 'approved' THEN v_at ELSE approved_at END,
      received_at = CASE WHEN v_step = 'received' THEN v_at ELSE received_at END,
      sent_to_supplier_at = CASE WHEN v_step = 'with_supplier' THEN v_at ELSE sent_to_supplier_at END,
      resolved_at = CASE WHEN v_step IN ('resolved', 'rejected') THEN v_at ELSE resolved_at END,
      updated_at = v_at
    WHERE id = v_id;
    v_prev := v_step;
  END LOOP;
END
$fn$;

-- contact, sample ticket (or NULL), source, status, outcome,
-- brand, product, qty, serials, fault,
-- invoice, bought, warranty, test result, supplier RMA ref, credit note, replacement order,
-- days ago, agent, note
DO $$
BEGIN
  PERFORM pg_temp.kd_sample_rma('orla.fitz@dublinfirecare.example', 'RMA request - smoke detectors DOA', 'agent', 'requested', NULL,
    'Ajax', 'FireProtect 2 (Heat/Smoke)', 3, E'FP2-41020\nFP2-41021\nFP2-41022', 'Dead on arrival - no LED on power-up, tried fresh batteries.',
    'INV-55821', CURRENT_DATE - 16, 'doa', NULL, NULL, NULL, NULL,
    0.8, 'tom.brennan@kd-demo.example', NULL);

  PERFORM pg_temp.kd_sample_rma('jim.hennessy.installs@gmail.com', NULL, 'form', 'requested', NULL,
    'Dahua', 'IPC-HDW2441T-ZS 4MP turret', 1, '8K02B9C4471ED', 'Night vision not working - picture goes black after dark.',
    '', NULL, NULL, NULL, NULL, NULL, NULL,
    0.15, NULL, NULL);

  PERFORM pg_temp.kd_sample_rma('liam.oconnor@atlanticfire.example', 'Warranty status - batch of PIRs', 'form', 'approved', NULL,
    'Ajax', 'MotionProtect', 4, E'MP-20A1F3\nMP-20A1F7\nMP-20A20B\nMP-20A214', 'Four PIRs bought in March stopped detecting within the last month.',
    'INV-54410', CURRENT_DATE - 250, 'in_warranty', NULL, NULL, NULL, NULL,
    2, 'aoife.byrne@kd-demo.example', NULL);

  PERFORM pg_temp.kd_sample_rma('sean.murphy@brightline-sec.example', 'NVR not recording after power cut', 'agent', 'received', NULL,
    'Dahua', 'NVR4216-16P-4KS2 16ch NVR', 1, 'NVR-16-2291', 'Stopped recording after a power cut; live view works, disk shows as abnormal.',
    'INV-53007', CURRENT_DATE - 140, 'in_warranty', 'fault_confirmed', NULL, NULL, NULL,
    4, 'aoife.byrne@kd-demo.example', NULL);

  PERFORM pg_temp.kd_sample_rma('emma.ryan@corribinstall.example', 'IP camera firmware update failed', 'agent', 'with_supplier', NULL,
    'Dahua', 'IPC-HFW2441S-S 4MP bullet', 1, 'IPC-4B-88120', 'Firmware update failed halfway; camera now boot-loops.',
    'INV-52288', CURRENT_DATE - 200, 'in_warranty', 'fault_confirmed', 'DH-EU-2611-0457', NULL, NULL,
    9, 'niamh.doyle@kd-demo.example', NULL);

  PERFORM pg_temp.kd_sample_rma('conor.daly@leesidealarms.example', NULL, 'form', 'resolved', 'credited',
    'Ajax', 'HomeSiren', 2, E'HS-118C02\nHS-118C05', 'Both sirens dead out of the box.',
    'INV-55102', CURRENT_DATE - 40, 'doa', 'fault_confirmed', NULL, 'CN-2231', NULL,
    21, 'ciaran.walsh@kd-demo.example', 'Credit note raised in Intact.');

  PERFORM pg_temp.kd_sample_rma('mark.hayes@shannonelec.example', NULL, 'agent', 'resolved', 'replaced',
    'Alta Labs', 'AP6-Pro access point', 1, 'AL6P-0091A3', 'Won''t power on from PoE on any switch.',
    'INV-54876', CURRENT_DATE - 90, 'in_warranty', 'fault_confirmed', 'ALT-RMA-31877', NULL, 'SO-10422',
    16, 'tom.brennan@kd-demo.example', 'Replacement sent from Cork stock.');

  PERFORM pg_temp.kd_sample_rma('sarah.quinn@leesidealarms.example', NULL, 'form', 'resolved', 'returned_nff',
    'Ajax', 'Hub 2 (4G)', 1, 'HUB2-77A0C1', 'Hub drops offline every evening.',
    'INV-53990', CURRENT_DATE - 120, 'in_warranty', 'no_fault_found', NULL, NULL, NULL,
    12, 'niamh.doyle@kd-demo.example', '72-hour soak test, no drop-outs. Advised checking the router''s DHCP lease.');

  PERFORM pg_temp.kd_sample_rma('liam.oconnor@atlanticfire.example', NULL, 'form', 'rejected', NULL,
    'Dahua', 'PFS3010-8ET-96 PoE switch', 1, 'PFS3010-55621', 'Two ports dead.',
    'INV-38412', CURRENT_DATE - 900, 'out_of_warranty', NULL, NULL, NULL, NULL,
    6, 'ciaran.walsh@kd-demo.example', 'Out of warranty - bought over two years ago. Offered a discounted replacement.');
END
$$;

COMMIT;
