<!-- KD: create or edit a customer (company). -->
<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{ customer?.id ? 'Edit customer' : 'New customer' }}</DialogTitle>
        <DialogDescription>
          A customer is a company. Link its contacts, and their tickets show up here.
        </DialogDescription>
      </DialogHeader>
      <form class="space-y-3" @submit.prevent="save">
        <div class="space-y-1.5">
          <Label for="kd-customer-name">Name</Label>
          <Input id="kd-customer-name" v-model="form.name" placeholder="Brightline Security" required />
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div class="space-y-1.5">
            <Label for="kd-customer-domain">Website / domain</Label>
            <Input id="kd-customer-domain" v-model="form.domain" placeholder="brightline.ie" />
          </div>
          <div class="space-y-1.5">
            <Label for="kd-customer-phone">Phone</Label>
            <Input id="kd-customer-phone" v-model="form.phone" placeholder="021 450 1100" />
          </div>
        </div>
        <div class="space-y-1.5">
          <Label for="kd-customer-notes">Notes</Label>
          <Textarea
            id="kd-customer-notes"
            v-model="form.notes"
            rows="3"
            placeholder="Account details, contract, anything agents should know"
          />
        </div>
        <DialogFooter class="pt-2">
          <Button type="button" variant="outline" @click="emit('update:open', false)">Cancel</Button>
          <Button type="submit" :disabled="saving || !form.name.trim()">
            {{ saving ? 'Saving…' : customer?.id ? 'Save' : 'Create customer' }}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>

<script setup>
import { reactive, ref, watch } from 'vue'
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
import { useEmitter } from '@main/composables/useEmitter'
import { EMITTER_EVENTS } from '@main/constants/emitterEvents'
import { invalidateCached } from '@main/composables/useCachedResource'
import api from '@main/api'

const props = defineProps({
  open: { type: Boolean, default: false },
  customer: { type: Object, default: null }
})
const emit = defineEmits(['update:open', 'saved'])
const emitter = useEmitter()
const saving = ref(false)
const form = reactive({ name: '', domain: '', phone: '', notes: '' })

watch(
  () => props.open,
  (open) => {
    if (!open) return
    const c = props.customer || {}
    Object.assign(form, {
      name: c.name || '',
      domain: c.domain || '',
      phone: c.phone || '',
      notes: c.notes || ''
    })
  },
  { immediate: true }
)

const save = async () => {
  saving.value = true
  try {
    const res = props.customer?.id
      ? await api.updateCustomer(props.customer.id, { ...form })
      : await api.createCustomer({ ...form })
    invalidateCached('customer')
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      description: props.customer?.id ? 'Customer saved' : 'Customer created'
    })
    emit('saved', res.data.data)
    emit('update:open', false)
  } catch (err) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(err).message
    })
  } finally {
    saving.value = false
  }
}
</script>
