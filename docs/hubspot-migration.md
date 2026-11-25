# Moving KD Helpdesk data into HubSpot

**Short answer:** everything the support team logs in KD Helpdesk (customers, contacts,
tickets with their full conversation, and RMAs) can be exported as four CSV files built
for HubSpot's import tool: **Dashboard → Export for HubSpot**. The move is a one-off at
switch-over, about half a day including checks, and nothing is lost.

## What goes where

| KD Helpdesk | HubSpot | Matched / linked by |
|---|---|---|
| Customers | **Companies** | website domain |
| Contacts | **Contacts**, linked to their company | email |
| Tickets | **Tickets** in the Support pipeline: subject, status, priority, owner, dates, and the whole conversation (with internal notes marked) in the description | contact's email, company's domain |
| RMAs | **Tickets** in a separate **RMA pipeline**, one stage per RMA step, with all RMA fields (serials, invoice, warranty, test result, supplier and their RMA number, outcome, credit note) as ticket properties | contact's email, company's domain |

Ticket statuses map onto HubSpot's default pipeline: Open / On-hold → *Waiting on us*,
Pending → *Waiting on contact*, Resolved / Closed → *Closed*. The original KD status and
ticket number are kept in their own columns.

## One-time setup in HubSpot (about an hour)

1. **RMA pipeline** (Settings → Objects → Tickets → Pipelines): stages *Requested,
   Approved, Received, With supplier, Resolved, Rejected*.
2. **Ticket properties** for the RMA fields: RMA number, Brand, Product, Quantity, Serial
   numbers, Invoice number, Purchase date, Warranty, Test result, Supplier, Supplier RMA
   number, Outcome, Credit note number, Replacement order number, Return tracking number,
   KD ticket number, KD status. Name them exactly like the CSV columns and the import maps
   itself.
3. **Users:** each support agent added in HubSpot with the same email as in the helpdesk,
   so tickets keep their owner.

If KD is on Service Hub Enterprise, RMAs could be a custom "RMA" object instead of a
pipeline. The pipeline works on every paid plan and is easy to change later.

## Switch-over day

1. Agree a cut-off time. Point the support email and the RMA form at HubSpot from then on.
2. Export the four files and import them **in order**: companies, contacts, tickets, RMAs.
3. Check the counts in HubSpot against the helpdesk dashboard, and open a few tickets and
   RMAs end to end.
4. Keep KD Helpdesk running read-only for a month as a fallback, then switch it off.

Try the import on a small file first (or a HubSpot sandbox): it shows any column HubSpot
doesn't recognise before the real run.

## Worth knowing

- **Conversations** come across as text in each ticket's description, not as separate
  emails. HubSpot's import tool can't recreate email threads; its API could, but for a
  one to two month bridge that effort isn't worth it.
- **Calls:** Nuacom has a HubSpot-certified integration, so once HubSpot is live, calls are
  logged against contacts and tickets automatically. Hand-logged call notes from this
  period come across in the conversation text.
- **The RMA form:** after the move, it either posts into HubSpot (the same Apps Script,
  pointed at HubSpot's API) or is replaced by the RMA section planned for the new KD
  website.
- **Attachments** aren't part of this prototype, so there are none to move.
