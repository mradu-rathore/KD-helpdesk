-- KD Helpdesk sample data (fictional people and companies only - no real KD data).
-- Safe to re-run: it removes the previous sample data first.
BEGIN;

-- Zendesk-style statuses and priority on top of Libredesk's defaults
INSERT INTO conversation_statuses (name, category) VALUES
  ('Pending', 'waiting'), ('On-hold', 'waiting')
ON CONFLICT (name) DO NOTHING;
INSERT INTO conversation_priorities (name) VALUES ('Urgent') ON CONFLICT (name) DO NOTHING;

-- Remove previous sample data (marked with the kd_sample flag)
DELETE FROM conversations WHERE meta->>'kd_sample' = 'true';
DELETE FROM users WHERE custom_attributes->>'kd_sample' = 'true';
DELETE FROM inboxes WHERE config->>'kd_sample' = 'true';
DELETE FROM teams WHERE name = 'Support' AND emoji = '🛠';

-- Inbox (disabled: no real mailbox connected)
INSERT INTO inboxes (name, channel, enabled, config, "from")
VALUES ('KD Support (sample)', 'email', false, '{"kd_sample": true}', 'support@kd-demo.example');

-- Team
INSERT INTO teams (name, emoji, conversation_assignment_type) VALUES ('Support', '🛠', 'Manual');

-- Agents (fictional; no password, so they can't log in)
INSERT INTO users (type, email, first_name, last_name, custom_attributes) VALUES
  ('agent', 'aoife.byrne@kd-demo.example',  'Aoife',  'Byrne',  '{"kd_sample": true}'),
  ('agent', 'ciaran.walsh@kd-demo.example', 'Ciarán', 'Walsh',  '{"kd_sample": true}'),
  ('agent', 'niamh.doyle@kd-demo.example',  'Niamh',  'Doyle',  '{"kd_sample": true}'),
  ('agent', 'tom.brennan@kd-demo.example',  'Tom',    'Brennan','{"kd_sample": true}');
INSERT INTO user_roles (user_id, role_id)
  SELECT u.id, r.id FROM users u, roles r
  WHERE u.custom_attributes->>'kd_sample' = 'true' AND u.type = 'agent' AND r.name = 'Agent';
INSERT INTO team_members (team_id, user_id)
  SELECT t.id, u.id FROM teams t, users u
  WHERE t.name = 'Support' AND t.emoji = '🛠' AND u.custom_attributes->>'kd_sample' = 'true' AND u.type = 'agent';

-- Customer contacts (fictional installers/distributors); company kept as an attribute for now
INSERT INTO users (type, email, first_name, last_name, phone_number, custom_attributes) VALUES
  ('contact', 'sean.murphy@brightline-sec.example',   'Seán',    'Murphy',   '0871234501', '{"kd_sample": true, "company": "Brightline Security"}'),
  ('contact', 'grace.kelly@brightline-sec.example',   'Grace',   'Kelly',    '0871234502', '{"kd_sample": true, "company": "Brightline Security"}'),
  ('contact', 'liam.oconnor@atlanticfire.example',    'Liam',    'O''Connor','0861234503', '{"kd_sample": true, "company": "Atlantic Fire Systems"}'),
  ('contact', 'emma.ryan@corribinstall.example',      'Emma',    'Ryan',     '0851234504', '{"kd_sample": true, "company": "Corrib Installations"}'),
  ('contact', 'conor.daly@leesidealarms.example',     'Conor',   'Daly',     '0831234505', '{"kd_sample": true, "company": "Leeside Alarms"}'),
  ('contact', 'sarah.quinn@leesidealarms.example',    'Sarah',   'Quinn',    '0831234506', '{"kd_sample": true, "company": "Leeside Alarms"}'),
  ('contact', 'mark.hayes@shannonelec.example',       'Mark',    'Hayes',    '0891234507', '{"kd_sample": true, "company": "Shannon Electrical"}'),
  ('contact', 'orla.fitz@dublinfirecare.example',     'Orla',    'Fitzgerald','0871234508','{"kd_sample": true, "company": "Dublin FireCare"}');

-- Tickets: (subject, first message, contact email, agent email or null, status, priority, created hours ago, agent reply?, internal note?)
DO $$
DECLARE
  inbox_id INT := (SELECT id FROM inboxes WHERE config->>'kd_sample' = 'true');
  team_id INT := (SELECT id FROM teams WHERE name = 'Support' AND emoji = '🛠');
  r RECORD;
  conv_id BIGINT;
  contact_id BIGINT;
  agent_id BIGINT;
  created TIMESTAMPTZ;
  last_at TIMESTAMPTZ;
  last_text TEXT;
  last_sender message_sender_type;
BEGIN
  FOR r IN SELECT * FROM (VALUES
    ('NVR not recording after power cut', 'Hi, our 16ch NVR at a client site stopped recording after a power cut last night. Live view works. Serial NVR-16-2291. Can you help?', 'sean.murphy@brightline-sec.example', 'aoife.byrne@kd-demo.example', 'Open', 'High', 3, false, true),
    ('Fire panel showing earth fault', 'Addressable fire panel (2 loop) showing an earth fault on loop 1 since this morning. Site is a school, need advice urgently.', 'liam.oconnor@atlanticfire.example', 'ciaran.walsh@kd-demo.example', 'Open', 'Urgent', 2, true, true),
    ('Intruder panel won''t arm - zone 4 tamper', 'Customer''s alarm panel will not arm, keypad shows tamper on zone 4. Replaced the detector already.', 'conor.daly@leesidealarms.example', null, 'Open', 'Medium', 5, false, false),
    ('IP camera firmware update failed', 'Firmware update on a 4MP bullet camera failed halfway, camera now boot-loops. Serial IPC-4B-88120.', 'emma.ryan@corribinstall.example', 'niamh.doyle@kd-demo.example', 'Pending', 'Medium', 26, true, false),
    ('RMA request - smoke detectors DOA', 'We received 10 optical smoke detectors on invoice INV-55821, 3 are dead on arrival. Please advise on RMA.', 'orla.fitz@dublinfirecare.example', 'tom.brennan@kd-demo.example', 'Open', 'High', 20, true, true),
    ('Access control reader not reading fobs', 'Proximity reader on main door stopped reading fobs after controller reboot. Other doors fine.', 'grace.kelly@brightline-sec.example', 'aoife.byrne@kd-demo.example', 'On-hold', 'Low', 50, true, true),
    ('Order query - missing PSU', 'Order SO-10233 arrived without the 12V 5A PSU that was on the invoice. Can you send it out?', 'mark.hayes@shannonelec.example', null, 'Open', 'Low', 8, false, false),
    ('Remote viewing app not connecting', 'Client can''t connect to their DVR from the mobile app since they changed broadband provider.', 'sarah.quinn@leesidealarms.example', 'ciaran.walsh@kd-demo.example', 'Resolved', 'Medium', 72, true, false),
    ('Before giving us a call - checklist question', 'Is there a checklist of info you need before we ring support? Want to brief our new engineers.', 'sean.murphy@brightline-sec.example', 'niamh.doyle@kd-demo.example', 'Closed', 'Low', 160, true, false),
    ('Hard drive recommendation for 32ch NVR', 'What surveillance-grade drives do you stock for a 32ch NVR, 30 days retention?', 'emma.ryan@corribinstall.example', 'tom.brennan@kd-demo.example', 'Resolved', 'Low', 96, true, false),
    ('Wireless sounder losing signal', 'Wireless external sounder keeps reporting loss of supervision, 25m from the panel through two walls.', 'conor.daly@leesidealarms.example', 'aoife.byrne@kd-demo.example', 'Pending', 'Medium', 30, true, false),
    ('Warranty status - batch of PIRs', 'Can you confirm warranty on PIRs bought in March? 4 have failed in the last month.', 'liam.oconnor@atlanticfire.example', null, 'Open', 'Medium', 1, false, false),
    ('Missed call - please ring back', 'Logged call: customer rang at 10:42 about a DVR password reset, wants a call back on 087 123 4501.', 'sean.murphy@brightline-sec.example', 'ciaran.walsh@kd-demo.example', 'Open', 'High', 0.5, false, true),
    ('Training session request', 'Would KD run a training session on the new access control range for our team in Galway?', 'mark.hayes@shannonelec.example', 'niamh.doyle@kd-demo.example', 'On-hold', 'Low', 120, true, false)
  ) AS t(subject, body, contact_email, agent_email, status, priority, hours_ago, has_reply, has_note)
  LOOP
    contact_id := (SELECT id FROM users WHERE email = r.contact_email AND type = 'contact');
    agent_id := (SELECT id FROM users WHERE email = r.agent_email AND type = 'agent');
    created := NOW() - make_interval(secs => r.hours_ago * 3600);
    last_at := created; last_text := r.body; last_sender := 'contact';

    INSERT INTO conversations (contact_id, assigned_user_id, assigned_team_id, inbox_id, status_id, priority_id,
                               subject, created_at, updated_at, meta)
    VALUES (contact_id, agent_id, team_id, inbox_id,
            (SELECT id FROM conversation_statuses WHERE name = r.status),
            (SELECT id FROM conversation_priorities WHERE name = r.priority),
            r.subject, created, created, '{"kd_sample": true}')
    RETURNING id INTO conv_id;

    INSERT INTO conversation_messages (type, status, conversation_id, content_type, content, text_content, sender_id, sender_type, created_at)
    VALUES ('incoming', 'received', conv_id, 'text', r.body, r.body, contact_id, 'contact', created);

    IF r.has_note THEN
      last_at := created + interval '10 minutes';
      INSERT INTO conversation_messages (type, status, private, conversation_id, content_type, content, text_content, sender_id, sender_type, created_at)
      VALUES ('outgoing', 'sent', true, conv_id, 'text',
              'Internal: checked order history in Intact - customer is on an installer account.',
              'Internal: checked order history in Intact - customer is on an installer account.',
              COALESCE(agent_id, (SELECT id FROM users WHERE email = 'aoife.byrne@kd-demo.example')), 'agent', last_at);
    END IF;

    IF r.has_reply THEN
      last_at := created + interval '45 minutes';
      last_text := 'Thanks for getting in touch. We''re looking into this and will update you shortly. - KD Support';
      last_sender := 'agent';
      INSERT INTO conversation_messages (type, status, conversation_id, content_type, content, text_content, sender_id, sender_type, created_at)
      VALUES ('outgoing', 'sent', conv_id, 'text', last_text, last_text, agent_id, 'agent', last_at);
    END IF;

    UPDATE conversations SET
      last_message = last_text, last_message_at = last_at, last_message_sender = last_sender,
      last_interaction = last_text, last_interaction_at = last_at, last_interaction_sender = last_sender,
      first_reply_at = CASE WHEN r.has_reply THEN last_at END,
      last_reply_at = CASE WHEN r.has_reply THEN last_at END,
      waiting_since = CASE WHEN NOT r.has_reply THEN created END,
      resolved_at = CASE WHEN r.status IN ('Resolved', 'Closed') THEN last_at + interval '1 hour' END,
      updated_at = last_at
    WHERE id = conv_id;
  END LOOP;
END $$;

COMMIT;
