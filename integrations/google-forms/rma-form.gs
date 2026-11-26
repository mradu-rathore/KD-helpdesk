/**
 * KD Helpdesk - RMA Google Form connector (Google Apps Script).
 *
 * Every time the RMA form is submitted, this sends the answers to KD Helpdesk,
 * which opens an RMA and a ticket for it. Responses still land in the form's
 * Google Sheet as before.
 *
 * Setup (details in README.md next to this file):
 *   1. Project Settings -> Script properties:
 *        HELPDESK_URL         e.g. https://kd-helpdesk.onrender.com
 *        HELPDESK_API_KEY     } from KD Helpdesk: Admin -> Agents -> "RMA Form"
 *        HELPDESK_API_SECRET  }   -> API key
 *   2. Run testConnection().
 *   3. Run createRmaForm() for a ready-made form, or set FORM_ID to an existing
 *      form's ID and run connectExistingForm().
 *
 * Safe to re-send: the helpdesk recognises a response it has already seen, so
 * sendAllResponses() never makes duplicate RMAs.
 */

// Which question feeds which RMA field, matched on the question's title
// (ignoring case and punctuation; the title may start with any of these).
// Checked in this order, so "Product brand" is a brand, not a product.
// Answers to any other question are kept on the ticket as they are.
const FIELD_QUESTIONS = [
  ['email', ['email', 'email address', 'your email']],
  ['name', ['your name', 'name', 'contact name', 'full name']],
  ['company', ['company', 'company name', 'business name', 'installer']],
  ['phone', ['phone', 'phone number', 'mobile', 'contact number']],
  ['invoice_number', ['invoice', 'invoice number', 'invoice or order number', 'order number', 'kd invoice']],
  ['purchase_date', ['purchase date', 'date of purchase', 'date purchased', 'when was it bought']],
  ['brand', ['brand', 'manufacturer', 'product brand']],
  ['serial_numbers', ['serial', 'serial number', 'serial numbers']],
  ['quantity', ['quantity', 'qty', 'how many']],
  ['product', ['product', 'product model', 'model', 'product code', 'item']],
  ['fault', ['fault', 'fault description', 'describe the fault', 'what is wrong', 'issue', 'problem', 'reason']],
];

const BRANDS = ['Dahua', 'Ajax', 'Alta Labs', 'Other'];

// ---------------------------------------------------------------------------
// Setup

/** Creates a ready-made RMA form with a response sheet, and connects it. */
function createRmaForm() {
  const form = FormApp.create('KD Electronics - RMA request');
  form.setDescription(
    'Request a return for a faulty or dead-on-arrival product. One form per product - ' +
    'if you are returning different products, send the form once for each. ' +
    'We will email you an RMA number and return instructions.'
  );
  form.setConfirmationMessage('Thanks - your RMA request is in. We will email you the RMA number shortly.');
  // Customers shouldn't need a Google account: no verified-email collection
  // (the form asks for an email itself) and no one-response limit - both force
  // sign-in. Also set Publish -> Responders -> "Anyone with the link" (README).
  form.setCollectEmail(false);
  form.setLimitOneResponsePerUser(false);
  try { form.setRequireLogin(false); } catch (err) { /* only exists on Workspace accounts */ }

  form.addTextItem().setTitle('Your name').setRequired(true);
  form.addTextItem().setTitle('Email').setRequired(true)
    .setValidation(FormApp.createTextValidation().requireTextIsEmail().build());
  form.addTextItem().setTitle('Company').setRequired(true);
  form.addTextItem().setTitle('Phone');

  form.addPageBreakItem().setTitle('The product');
  form.addListItem().setTitle('Brand').setChoiceValues(BRANDS).setRequired(true);
  form.addTextItem().setTitle('Product (model or code)').setHelpText('e.g. IPC-HFW2441S-S').setRequired(true);
  form.addTextItem().setTitle('Quantity').setRequired(true)
    .setValidation(FormApp.createTextValidation().requireWholeNumber().build());
  form.addParagraphTextItem().setTitle('Serial numbers').setHelpText('One per line');
  form.addTextItem().setTitle('Invoice or order number').setHelpText('From your KD invoice, if you have it');
  form.addDateItem().setTitle('Purchase date');
  form.addParagraphTextItem().setTitle('Describe the fault').setRequired(true);

  const sheet = SpreadsheetApp.create('KD Electronics - RMA requests (responses)');
  form.setDestination(FormApp.DestinationType.SPREADSHEET, sheet.getId());

  PropertiesService.getScriptProperties().setProperty('FORM_ID', form.getId());
  installTrigger_(form);
  Logger.log('Form for customers:  ' + form.getPublishedUrl());
  Logger.log('Edit the form:       ' + form.getEditUrl());
  Logger.log('Responses sheet:     ' + sheet.getUrl());
}

/** Connects an existing form: set the FORM_ID script property first. */
function connectExistingForm() {
  const form = getForm_();
  installTrigger_(form);
  const titles = form.getItems().map((i) => i.getTitle()).filter(Boolean);
  Logger.log('Connected "' + form.getTitle() + '". Its questions map like this:');
  titles.forEach((t) => Logger.log('  ' + t + '  ->  ' + (fieldFor_(t) || '(kept on the ticket as is)')));
}

/** Checks the helpdesk address and API key. */
function testConnection() {
  const res = call_('get', '/api/v1/kd/rmas?page_size=1');
  if (res.code === 200) Logger.log('Connected to KD Helpdesk.');
  else Logger.log('Could not connect (HTTP ' + res.code + '): ' + res.message);
}

/**
 * Sends every response the form has, e.g. to import requests from before the
 * connection, or after the helpdesk was unreachable. Already-sent ones are skipped.
 */
function sendAllResponses() {
  const responses = getForm_().getResponses();
  let created = 0, already = 0, failed = 0;
  responses.forEach((r) => {
    const result = send_(r);
    if (!result.ok) failed++;
    else if (result.duplicate) already++;
    else created++;
  });
  Logger.log(responses.length + ' responses: ' + created + ' new RMAs, ' + already + ' already in, ' + failed + ' failed.');
}

// ---------------------------------------------------------------------------
// The trigger

/** Runs on every form submission (installed by createRmaForm / connectExistingForm). */
function onRmaFormSubmit(e) {
  const result = send_(e.response);
  if (!result.ok) {
    // Logged in Executions. Fix the cause, then run sendAllResponses() to catch up.
    throw new Error('RMA not sent to KD Helpdesk: ' + result.message);
  }
}

// ---------------------------------------------------------------------------
// Helpers

function send_(response) {
  const payload = toPayload_(response);
  // The helpdesk may be waking up (free hosting sleeps when idle): try a few times.
  let res;
  for (let attempt = 1; attempt <= 3; attempt++) {
    res = call_('post', '/api/v1/kd/rmas/intake', payload);
    if (res.code === 200) {
      const data = res.body.data;
      Logger.log((data.duplicate ? 'Already in: ' : 'Created: ') + data.rma.number + ' (response ' + payload.response_id + ')');
      return { ok: true, duplicate: data.duplicate };
    }
    if (res.code === 400 || res.code === 401 || res.code === 403) break; // won't fix itself
    Utilities.sleep(5000 * attempt);
  }
  Logger.log('Failed for response ' + payload.response_id + ' (HTTP ' + res.code + '): ' + res.message);
  return { ok: false, message: res.message };
}

function toPayload_(response) {
  const payload = { response_id: response.getId(), submitted_at: response.getTimestamp().toISOString(), other: [] };
  response.getItemResponses().forEach((ir) => {
    const title = ir.getItem().getTitle();
    const answer = answerText_(ir.getResponse());
    if (!answer) return;
    const field = fieldFor_(title);
    if (field && !payload[field]) payload[field] = answer;
    else payload.other.push([title, answer]);
  });
  // Forms set to collect email addresses give it here instead of as a question.
  if (!payload.email) {
    try { payload.email = response.getRespondentEmail(); } catch (err) { /* not collected */ }
  }
  return payload;
}

function answerText_(value) {
  if (value === null || value === undefined) return '';
  if (Array.isArray(value)) return value.map(answerText_).filter(Boolean).join(', ');
  return String(value).trim();
}

function fieldFor_(title) {
  const t = normalise_(title);
  for (const [field, names] of FIELD_QUESTIONS) {
    if (names.some((n) => t === normalise_(n) || t.indexOf(normalise_(n) + ' ') === 0)) return field;
  }
  return '';
}

function normalise_(s) {
  return String(s).toLowerCase().replace(/[^a-z0-9]+/g, ' ').trim();
}

function call_(method, path, payload) {
  const props = PropertiesService.getScriptProperties();
  const url = (props.getProperty('HELPDESK_URL') || '').replace(/\/+$/, '');
  const key = props.getProperty('HELPDESK_API_KEY');
  const secret = props.getProperty('HELPDESK_API_SECRET');
  if (!url || !key || !secret) {
    throw new Error('Set HELPDESK_URL, HELPDESK_API_KEY and HELPDESK_API_SECRET in Project Settings -> Script properties.');
  }
  const options = {
    method: method,
    headers: { Authorization: 'token ' + key + ':' + secret },
    muteHttpExceptions: true,
  };
  if (payload) {
    options.contentType = 'application/json';
    options.payload = JSON.stringify(payload);
  }
  try {
    const res = UrlFetchApp.fetch(url + path, options);
    let body = {};
    try { body = JSON.parse(res.getContentText()); } catch (err) { /* not JSON */ }
    return { code: res.getResponseCode(), body: body, message: body.message || res.getContentText().slice(0, 200) };
  } catch (err) {
    return { code: 0, body: {}, message: String(err) };
  }
}

function getForm_() {
  const id = PropertiesService.getScriptProperties().getProperty('FORM_ID');
  if (!id) throw new Error('Set the FORM_ID script property (the long ID in the form\'s edit link).');
  return FormApp.openById(id);
}

function installTrigger_(form) {
  ScriptApp.getProjectTriggers()
    .filter((t) => t.getHandlerFunction() === 'onRmaFormSubmit')
    .forEach((t) => ScriptApp.deleteTrigger(t));
  ScriptApp.newTrigger('onRmaFormSubmit').forForm(form).onFormSubmit().create();
}
