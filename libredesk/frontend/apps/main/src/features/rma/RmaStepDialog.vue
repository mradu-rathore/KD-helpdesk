<!--
  KD: the two RMA steps that need more than a click - resolving (pick the
  outcome, plus the credit note / replacement order number) and rejecting
  (say why). Emits { status, outcome, note, fields } for the page to save.
-->
<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="sm:max-w-lg">
      <DialogHeader>
        <DialogTitle>{{ mode === 'reject' ? `Reject ${rma?.number}` : `Resolve ${rma?.number}` }}</DialogTitle>
        <DialogDescription>
          {{ mode === 'reject'
            ? 'The reason goes in the RMA history and on the ticket. Reply to the customer on the ticket to let them know.'
            : 'How was it settled? This closes the RMA; you can reopen it later.' }}
        </DialogDescription>
      </DialogHeader>

      <form class="space-y-4" @submit.prevent="submit">
        <fieldset v-if="mode === 'resolve'" class="grid gap-2 sm:grid-cols-2">
          <legend class="sr-only">Outcome</legend>
          <label
            v-for="o in RMA_OUTCOMES"
            :key="o.key"
            class="flex cursor-pointer items-start gap-2.5 rounded-lg border p-3 transition-colors"
            :class="outcome === o.key ? 'border-foreground bg-accent/50' : 'border-border hover:bg-accent/30'"
          >
            <input v-model="outcome" type="radio" name="kd-rma-outcome" :value="o.key" class="mt-0.5 accent-foreground" />
            <span>
              <span class="block text-sm font-medium">{{ o.label }}</span>
              <span class="block text-xs text-muted-foreground">{{ o.hint }}</span>
            </span>
          </label>
        </fieldset>

        <div v-if="outcomeField" class="space-y-1.5">
          <Label for="kd-rma-outcome-field">{{ outcomeField.fieldLabel }}</Label>
          <Input id="kd-rma-outcome-field" v-model="fieldValue" :placeholder="outcomeField.key === 'credited' ? 'CN-2231' : 'SO-10422'" />
          <p class="text-xs text-muted-foreground">From Intact. Optional now - you can add it later.</p>
        </div>

        <div class="space-y-1.5">
          <Label for="kd-rma-step-note">{{ mode === 'reject' ? 'Reason' : 'Note' }} <span v-if="mode !== 'reject'" class="font-normal text-muted-foreground">(optional)</span></Label>
          <Textarea
            id="kd-rma-step-note"
            v-model="note"
            rows="3"
            :placeholder="mode === 'reject' ? 'e.g. Out of warranty - bought March 2023' : 'Anything worth keeping on record'"
          />
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" @click="emit('update:open', false)">Cancel</Button>
          <Button type="submit" :variant="mode === 'reject' ? 'destructive' : 'default'" :disabled="!canSubmit || busy">
            {{ mode === 'reject' ? 'Reject RMA' : 'Resolve RMA' }}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
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
import { RMA_OUTCOMES } from '@main/constants/kdRmas'

const props = defineProps({
  open: Boolean,
  mode: { type: String, default: 'resolve' }, // 'resolve' | 'reject'
  rma: { type: Object, default: null },
  busy: Boolean
})
const emit = defineEmits(['update:open', 'submit'])

const outcome = ref('')
const note = ref('')
const fieldValue = ref('')

watch(
  () => props.open,
  (open) => {
    if (!open) return
    outcome.value = props.rma?.outcome || ''
    note.value = ''
    fieldValue.value = ''
  }
)

const outcomeField = computed(() =>
  props.mode === 'resolve' ? RMA_OUTCOMES.find((o) => o.key === outcome.value && o.field) : null
)
// Show what's already on the RMA for the chosen outcome's field.
watch(outcomeField, (f) => {
  fieldValue.value = f ? props.rma?.[f.field] || '' : ''
})

const canSubmit = computed(() => (props.mode === 'reject' ? note.value.trim() !== '' : outcome.value !== ''))

function submit () {
  if (!canSubmit.value) return
  const fields = outcomeField.value ? { [outcomeField.value.field]: fieldValue.value.trim() } : {}
  emit('submit', {
    status: props.mode === 'reject' ? 'rejected' : 'resolved',
    outcome: props.mode === 'reject' ? '' : outcome.value,
    note: note.value.trim(),
    fields
  })
}
</script>
