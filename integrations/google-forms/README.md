# RMA Google Form → KD Helpdesk

When someone submits the RMA form, the helpdesk opens an **RMA** and a **ticket** for it
within a few seconds. The sender is matched to a contact by email, and to a customer by
the contact's link, their email domain or the company name. If nothing matches, the RMA
shows **No customer** for an agent to link. Responses still go to the form's Google
Sheet as before.

[`rma-form.gs`](rma-form.gs) is a Google Apps Script that sends each submission to
`POST /api/v1/kd/rmas/intake`. It signs in with the API key of an agent kept just for
the form. Sending the same response twice is safe: the helpdesk returns the RMA it made
the first time.

The helpdesk must be reachable from the internet (Google calls it), so use the hosted
copy (Render), not `localhost`.

## KD's demo form (live)

Created on 26 Nov 2026 with `createRmaForm()`, owned by **kd.support.demo@gmail.com**,
connected to the hosted helpdesk (https://kd-helpdesk.onrender.com).

| What | Link |
|---|---|
| Form for customers (submit RMAs here) | https://docs.google.com/forms/d/e/1FAIpQLSeFwKpS3svS6OQCiKksZv74IQo3KYDtZGXWcLW7QVhQdGL5PA/viewform |
| Edit the form | https://docs.google.com/forms/d/1G3ADdBL-BIwSyy7I602ZUkGhtTeIKBNdcEInYbiRiLU/edit |
| Responses sheet | https://docs.google.com/spreadsheets/d/1jN5OTzQ2SP3WTLqyoId0hhrYJJiJa11xFDI5Y0q0wTQ/edit |

The edit link and the sheet open only for the owner account (or people it's shared
with); the customer link works for anyone. The script is the Apps Script project
**KD RMA form connector** in the same account.

## 1. In KD Helpdesk: an agent for the form

1. **Admin → Agents → New agent**: first name `RMA`, last name `Form`, any email (e.g.
   `rma-form@kdelectronics.ie`), role **Agent**, no team. Don't set a password - nobody
   signs in as it.
2. Open the agent, and under **API keys** click generate. Copy the **key** and the
   **secret** (the secret is shown once).

Form submissions show in RMA history and ticket timelines as "RMA Form".

## 2. In Google: the script

1. Go to [script.google.com](https://script.google.com) with the Google account that
   should own the form → **New project**. Name it `KD RMA form connector`.
2. Replace the contents of `Code.gs` with [`rma-form.gs`](rma-form.gs). Save.
3. **Project Settings** (gear) → **Script properties** → add:

   | Property | Value |
   |---|---|
   | `HELPDESK_URL` | the helpdesk address, e.g. `https://kd-helpdesk.onrender.com` |
   | `HELPDESK_API_KEY` | the key from step 1 |
   | `HELPDESK_API_SECRET` | the secret from step 1 |

4. Back in the editor, pick **testConnection** in the function menu → **Run**. Google
   asks you to authorise the script the first time: it needs to manage forms and
   sheets, connect to the helpdesk, and run when the form is submitted. The log should
   say `Connected to KD Helpdesk.`
5. Choose one:
   - **A new form**: run **createRmaForm**. The log shows the link to send customers,
     the edit link and the responses sheet. Then open the edit link → **Published /
     Publish → Manage → Responders: Anyone with the link**, so customers don't need a
     Google account (check by opening the customer link in an incognito window).
   - **KD's existing RMA form**: add a script property `FORM_ID` (the long ID in the
     form's edit link, between `/d/` and `/edit`), then run **connectExistingForm**. The
     log lists each question and which RMA field it fills. To import requests made
     before connecting, run **sendAllResponses**.

That's it: submit the form and the RMA appears under **RMAs** (and the ticket in
**Unassigned tickets**).

## Question → field matching

The script matches questions by title, ignoring case and punctuation (see
`FIELD_QUESTIONS` at the top of the script):

| RMA field | Question titles it recognises |
|---|---|
| email (required) | Email, Email address - or the form's own "collect email addresses" |
| name, company, phone | Your name / Name, Company / Company name, Phone / Mobile |
| brand | Brand, Manufacturer |
| product (required) | Product, Product (model or code), Model |
| quantity | Quantity, Qty, How many |
| serial numbers | Serial number(s) |
| invoice | Invoice number, Invoice or order number, Order number |
| purchase date | Purchase date, Date of purchase |
| fault | Describe the fault, Fault, Issue, Problem, Reason for return |

Answers to any other question are kept on the ticket under their question.

## Before a demo

- The free Render plan sleeps after 15 minutes idle, and the first request then takes
  about a minute. Open the helpdesk in a browser a few minutes before, so it's awake.
  The script retries for a while, and `sendAllResponses` catches up anything missed.
- Have two tabs ready: the form (customer view) and **RMAs** in the helpdesk. Submit,
  then refresh RMAs: the new RMA is under **Requested**, with its ticket.

## When something goes wrong

The script's **Executions** page (left bar) shows each run and its log.

| Log says | Fix |
|---|---|
| HTTP 401 | Key or secret wrong, or the key was regenerated: copy them again. |
| HTTP 403 | The form's agent is missing the **Agent** role. |
| HTTP 400 "A valid email address is required" / "Add the product" | The form has no question the script recognises as email / product: rename the question or add its title to `FIELD_QUESTIONS`. |
| HTTP 0 or 5xx | Helpdesk asleep or down. Once it's up, run `sendAllResponses`. |
| Customers are asked to sign in to Google | In the form: **Publish → Manage → Responders: Anyone with the link**; **Settings → Responses**: Collect email addresses **Do not collect**, Limit to 1 response **off**. |

## Local testing

Google can't reach `localhost`. Use `dev/rma-form-key.sh` (creates the form's agent and
key locally), then `dev/rma-intake-test.sh` to send fake submissions exactly as the
script does.
