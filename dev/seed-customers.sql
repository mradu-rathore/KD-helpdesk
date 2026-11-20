-- KD sample customers (fictional), built from the sample contacts' "company"
-- attribute. Each contact is linked to its customer, so their tickets show it.
-- Re-runnable: replaces earlier sample customers. Needs the kd_customers tables,
-- which the app creates on start / --upgrade.

DELETE FROM kd_customers WHERE kd_sample;

INSERT INTO kd_customers (name, domain, phone, notes, kd_sample) VALUES
  ('Brightline Security',   'brightline-sec.example',  '021 450 1100', 'Installer, Cork. Trade account since 2019.', true),
  ('Atlantic Fire Systems', 'atlanticfire.example',    '091 560 2200', 'Fire alarm specialist, Galway.', true),
  ('Corrib Installations',  'corribinstall.example',   '091 730 3300', 'Small installer, mostly domestic intruder.', true),
  ('Leeside Alarms',        'leesidealarms.example',   '021 490 4400', 'Key account - priority support.', true),
  ('Shannon Electrical',    'shannonelec.example',     '061 320 5500', 'Electrical wholesaler, Limerick.', true),
  ('Dublin FireCare',       'dublinfirecare.example',  '01 870 6600',  'Fire maintenance contractor.', true)
ON CONFLICT (LOWER(name)) DO NOTHING;

INSERT INTO kd_contact_customers (contact_id, customer_id)
SELECT u.id, k.id
FROM users u JOIN kd_customers k ON LOWER(k.name) = LOWER(u.custom_attributes->>'company')
WHERE u.type = 'contact' AND u.custom_attributes ? 'company'
ON CONFLICT (contact_id) DO UPDATE SET customer_id = EXCLUDED.customer_id;
