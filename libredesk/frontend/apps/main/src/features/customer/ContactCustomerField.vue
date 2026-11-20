<!-- KD: the contact's customer (company), with a picker to link or change it. -->
<template>
  <div class="max-w-xs pt-1">
    <p class="mb-1 text-xs font-medium text-muted-foreground">Customer</p>
    <CustomerPicker
      v-if="canEdit"
      :model-value="customer"
      placeholder="Link to a customer"
      none-label="Remove from customer"
      @select="select"
    />
    <router-link
      v-else-if="customer"
      :to="{ name: 'customer-detail', params: { id: customer.id } }"
      class="text-sm hover:underline"
    >
      {{ customer.name }}
    </router-link>
    <p v-else class="text-sm text-muted-foreground">—</p>
    <router-link
      v-if="customer && canEdit"
      :to="{ name: 'customer-detail', params: { id: customer.id } }"
      class="mt-1 inline-block text-xs text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
    >
      View {{ customer.name }}
    </router-link>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import CustomerPicker from './CustomerPicker.vue'
import { invalidateCached } from '@main/composables/useCachedResource'
import { useEmitter } from '@main/composables/useEmitter'
import { EMITTER_EVENTS } from '@main/constants/emitterEvents'
import { useUserStore } from '@main/stores/user'
import api from '@main/api'

const props = defineProps({ contactId: { type: [Number, String], required: true } })
const emitter = useEmitter()
const userStore = useUserStore()
const canEdit = computed(() => userStore.can('contacts:write'))
const customer = ref(null)

watch(
  () => props.contactId,
  async (id) => {
    customer.value = null
    try {
      customer.value = (await api.getContactCustomer(id)).data.data || null
    } catch {
      customer.value = null
    }
  },
  { immediate: true }
)

const select = async (next) => {
  const previous = customer.value
  customer.value = next
  try {
    if (next) await api.linkCustomerContact(next.id, props.contactId)
    else if (previous) await api.unlinkCustomerContact(previous.id, props.contactId)
    invalidateCached('customer')
    invalidateCached('contacts')
  } catch (err) {
    customer.value = previous
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, { variant: 'destructive', description: handleHTTPError(err).message })
  }
}
</script>
