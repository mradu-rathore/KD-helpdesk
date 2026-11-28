-- KD sample phone calls (fictional data only) on the sample tickets: incoming
-- and outgoing, one call-back waiting and one made, and "Missed call - please
-- ring back" as a ticket that started on the phone.
-- Re-runnable: replaces earlier sample calls. Needs seed.sql first and the
-- calls table (the app creates it on start / --upgrade).
BEGIN;

DELETE FROM kd_calls WHERE kd_sample;
DELETE FROM conversation_messages WHERE meta->>'kd_sample_call' = 'true';

CREATE FUNCTION pg_temp.kd_sample_call(
  p_ticket TEXT,        -- sample ticket subject
  p_agent TEXT,         -- agent email
  p_direction TEXT,
  p_mins INT,
  p_hours_ago NUMERIC,
  p_notes TEXT,
  p_callback BOOL,
  p_callback_done_by TEXT,  -- agent email, or NULL if still waiting
  p_as_description BOOL     -- the call is the ticket's first message (ticket started on the phone)
) RETURNS void LANGUAGE plpgsql AS $fn$
DECLARE
  v_conv BIGINT; v_contact BIGINT; v_phone TEXT; v_agent BIGINT; v_msg BIGINT; v_html TEXT;
  v_at TIMESTAMPTZ := NOW() - make_interval(secs => (p_hours_ago * 3600)::int);
BEGIN
  SELECT c.id, c.contact_id, TRIM(COALESCE(u.phone_number_country_code, '') || ' ' || COALESCE(u.phone_number, ''))
  INTO v_conv, v_contact, v_phone
  FROM conversations c JOIN users u ON u.id = c.contact_id
  WHERE c.subject = p_ticket AND c.meta->>'kd_sample' = 'true' LIMIT 1;
  SELECT id INTO v_agent FROM users WHERE email = p_agent AND type = 'agent';

  v_html := '<p><strong>📞 ' || CASE WHEN p_direction = 'outgoing' THEN 'Outgoing' ELSE 'Incoming' END || ' call</strong>'
    || CASE WHEN p_mins > 0 THEN ' · ' || p_mins || ' min' ELSE '' END
    || CASE WHEN v_phone <> '' THEN ' · ' || v_phone ELSE '' END
    || '</p><p>' || p_notes || '</p>'
    || CASE WHEN p_callback THEN '<p><strong>Call back needed</strong></p>' ELSE '' END;

  IF p_as_description THEN
    -- The ticket started with this call: its first message becomes the call
    UPDATE conversations SET meta = meta || '{"kd_source": "phone"}' WHERE id = v_conv;
    SELECT id INTO v_msg FROM conversation_messages WHERE conversation_id = v_conv ORDER BY id LIMIT 1;
    UPDATE conversation_messages SET content = v_html, content_type = 'html',
      text_content = regexp_replace(replace(v_html, '</p>', E'\n'), '<[^>]+>', '', 'g')
    WHERE id = v_msg;
    SELECT created_at INTO v_at FROM conversation_messages WHERE id = v_msg;
  ELSE
    INSERT INTO conversation_messages (type, status, private, conversation_id, content_type, content, text_content,
                                       sender_id, sender_type, created_at, meta)
    VALUES ('outgoing', 'sent', true, v_conv, 'html', v_html,
            regexp_replace(replace(v_html, '</p>', E'\n'), '<[^>]+>', '', 'g'),
            v_agent, 'agent', v_at, '{"kd_sample_call": true}')
    RETURNING id INTO v_msg;
  END IF;

  INSERT INTO kd_calls (created_at, called_at, conversation_id, contact_id, user_id, direction, phone, duration_mins,
                        notes, callback_needed, callback_done_at, callback_done_by, message_id, kd_sample)
  VALUES (v_at, v_at, v_conv, v_contact, v_agent, p_direction, v_phone, NULLIF(p_mins, 0), p_notes, p_callback,
          CASE WHEN p_callback_done_by IS NOT NULL THEN v_at + interval '1 hour' END,
          (SELECT id FROM users WHERE email = p_callback_done_by AND type = 'agent'),
          v_msg, true);
END
$fn$;

DO $$
BEGIN
  PERFORM pg_temp.kd_sample_call('Missed call - please ring back', 'ciaran.walsh@kd-demo.example', 'incoming', 2, 0,
    'Seán rang about a DVR password reset at a client site, wants a call back on his mobile.', true, NULL, true);
  PERFORM pg_temp.kd_sample_call('NVR not recording after power cut', 'aoife.byrne@kd-demo.example', 'incoming', 6, 2.5,
    'Seán rang to chase - still not recording. Disk shows as abnormal in the NVR menu; asked for the disk serial and raised an RMA.', false, NULL, false);
  PERFORM pg_temp.kd_sample_call('Fire panel showing earth fault', 'ciaran.walsh@kd-demo.example', 'outgoing', 8, 1.5,
    'Talked Liam through isolating loop 1 - earth fault traced to the zone 3 detector head. He is swapping it this afternoon.', false, NULL, false);
  PERFORM pg_temp.kd_sample_call('Intruder panel won''t arm - zone 4 tamper', 'aoife.byrne@kd-demo.example', 'incoming', 3, 4,
    'Conor rang: tamper on zone 4 even with a new detector. Needs the wiring diagram for the expander - promised a call back.', true, 'tom.brennan@kd-demo.example', false);
  PERFORM pg_temp.kd_sample_call('Remote viewing app not connecting', 'ciaran.walsh@kd-demo.example', 'incoming', 12, 70,
    'Sarah rang - client changed broadband. Walked her through re-adding the device by serial on the app; working again.', false, NULL, false);
END
$$;

COMMIT;
