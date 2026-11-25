<!--
  KD: raise an RMA. From a ticket it goes on that ticket; from the RMAs page
  it opens a new ticket for the chosen customer/contact (nothing is emailed).
-->
<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="max-h-[90vh] overflow-y-auto sm:max-w-xl">
      <DialogHeader>
        <DialogTitle>New RMA</DialogTitle>
        <DialogDescription>
          <template v-if="ticket">On ticket #{{ ticket.reference_number }}. One product per RMA - add another for a different product.</template>
          <template v-else>A ticket is opened for it too, so you can reply to the customer there.</template>
        </DialogDescription>
      </DialogHeader>

      <form class="space-y-3" novalidate @submit.prevent="save">
        <div v-if="!ticket" class="grid gap-3 sm:grid-cols-2">
          <div class="space-y-1.5">
            <Label>Customer</Label>
            <CustomerPicker :model-value="form.customer" placeholder="Choose customer" @select="selectCustomer" />
          </div>
          <div class="space-y-1.5">
            <Label>Contact <span class="font-normal text-muted-foreground">(optional)</span></Label>
            <ContactPicker
              :model-value="form.contact"
              :customer-id="form.customer?.id || 0"
              placeholder="Who is returning it?"
              @select="selectContact"
            />
          </div>
        </div>

        <div class="grid gap-3 sm:grid-cols-[9rem_1fr_5rem]">
          <div class="space-y-1.5">
            <Label for="kd-rma-brand">Brand</Label>
            <Input id="kd-rma-brand" v-model="form.brand" list="kd-rma-brands" placeholder="Dahua" />
            <datalist id="kd-rma-brands">
              <option v-for="b in RMA_BRANDS" :key="b" :value="b" />
            </datalist>
          </div>
          <div class="space-y-1.5">
            <Label for="kd-rma-product">Product (model or code)</Label>
            <Input id="kd-rma-product" v-model="form.product" placeholder="IPC-HFW2441S-S" />
          </div>
          <div class="space-y-1.5">
            <Label for="kd-rma-qty">Qty</Label>
            <Input id="kd-rma-qty" v-model.number="form.quantity" type="number" min="1" />
          </div>
        </div>

        <div class="grid gap-3 sm:grid-cols-2">
          <div class="space-y-1.5">
            <Label for="kd-rma-serials">Serial numbers</Label>
            <Textarea id="kd-rma-serials" v-model="form.serial_numbers" rows="3" placeholder="One per line" />
          </div>
          <div class="space-y-1.5">
            <Label for="kd-rma-fault">Fault</Label>
            <Textarea id="kd-rma-fault" v-model="form.fault" rows="3" placeholder="What's wrong with it?" />
          </div>
        </div>

        <div class="grid gap-3 sm:grid-cols-3">
          <div class="space-y-1.5">
            <Label for="kd-rma-invoice">Invoice / order no.</Label>
            <Input id="kd-rma-invoice" v-model="form.invoice_number" placeholder="INV-55821" />
          </div>
          <div class="space-y-1.5">
            <Label for="kd-rma-date">Purchase date</Label>
            <Input id="kd-rma-date" v-model="form.purchase_date" type="date" :max="today" />
          </div>
          <div class="space-y-1.5">
            <Label for="kd-rma-warranty">Warranty</Label>
            <select id="kd-rma-warranty" v-model="form.warranty" :class="[SELECT_CLASS, 'w-full']">
              <option value="">Not checked yet</option>
              <option v-for="w in RMA_WARRANTY" :key="w.key" :value="w.key">{{ w.label }}</option>
            </select>
          </div>
        </div>

        <DialogFooter class="pt-2">
          <Button type="button" variant="outline" @click="emit('update:open', false)">Cancel</Button>
          <Button type="submit" :disabled="saving">{{ saving ? 'Creating…' : 'Create RMA' }}</Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>

<script setup>
import { reactive, ref, watch } from 'vue'
import { format } from 'date-fns'
import { Button } from '@shared-ui/components/ui/button'
import { Input } from '@shared-ui/components/ui/input'
import { Label } from '@shared-ui/components/ui/label'
import { Textarea } from '@shared-ui/components/ui/textarea'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@shared-ui/components/ui/dialog'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import CustomerPicker from '@/features/customer/CustomerPicker.vue'
import ContactPicker from '@/features/customer/ContactPicker.vue'
import { useEmitter } from '@main/composables/useEmitter'
import { invalidateCached } from '@main/composables/useCachedResource'
import { EMITTER_EVENTS } from '@main/constants/emitterEvents'
import { RMA_BRANDS, RMA_WARRANTY, SELECT_CLASS } from '@main/constants/kdRmas'
import api from '@main/api'

const props = defineProps({
  open: Boolean,
  // { uuid, reference_number } to raise it on an existing ticket
  ticket: { type: Object, default: null },
  // preselected customer when raised from a customer page
  customer: { type: Object, default: null }
})
const emit = defineEmits(['update:open', 'saved'])

const emitter = useEmitter()
const saving = ref(false)
const today = format(new Date(), 'yyyy-MM-dd')

const blank = () => ({
  customer: props.customer,
  contact: null,
  brand: '',
  product: '',
  quantity: 1,
  serial_numbers: '',
  fault: '',
  invoice_number: '',
  purchase_date: '',
  warranty: ''
})
const form = reactive(blank())
watch(
  () => props.open,
  (open) => open && Object.assign(form, blank())
)

const selectCustomer = (customer) => {
  form.customer = customer
  if (customer && form.contact?.customer_id && form.contact.customer_id !== customer.id) form.contact = null
}
const selectContact = (contact) => {
  form.contact = contact
  if (contact?.customer_id && !form.customer) form.customer = { id: contact.customer_id, name: contact.customer_name }
}

const toast = (description, variant) => emitter.emit(EMITTER_EVENTS.SHOW_TOAST, { description, variant })

async function save () {
  if (saving.value) return
  const error = !props.ticket && !form.customer && !form.contact
    ? 'Choose a customer or a contact'
    : !form.product.trim()
      ? 'Add the product'
      : ''
  if (error) return toast(error, 'destructive')

  saving.value = true
  try {
    const res = await api.createRma({
      conversation_uuid: props.ticket?.uuid || '',
      customer_id: form.customer?.id || 0,
      contact_id: form.contact?.id || 0,
      brand: form.brand,
      product: form.product,
      quantity: Number(form.quantity) || 1,
      serial_numbers: form.serial_numbers,
      fault: form.fault,
      invoice_number: form.invoice_number,
      purchase_date: form.purchase_date,
      warranty: form.warranty
    })
    const rma = res.data.data
    invalidateCached('rma')
    invalidateCached('customer')
    toast(`${rma.number} created`)
    emit('update:open', false)
    emit('saved', rma)
  } catch (err) {
    toast(handleHTTPError(err).message, 'destructive')
  } finally {
    saving.value = false
  }
}
</script>
