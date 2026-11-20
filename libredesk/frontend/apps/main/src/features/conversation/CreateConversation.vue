<template>
  <div
    v-if="dialogOpen && !minimized"
    class="fixed inset-0 z-50 bg-black/40 animate-in fade-in-0 max-sm:hidden"
    @click="minimized = true"
  />
  <section
    v-if="dialogOpen"
    role="dialog"
    :aria-label="$t('conversation.newConversation')"
    :class="panelClass"
  >
    <header class="flex items-center gap-1 border-b border-border bg-muted pl-4 pr-1 h-11 shrink-0">
      <button
        type="button"
        class="flex-1 min-w-0 truncate text-left text-sm font-medium"
        @click="minimized = !minimized"
      >
        {{ $t('conversation.newConversation') }}
      </button>
      <Badge v-if="minimized && hasDraft" variant="secondary" class="shrink-0">
        {{ $t('globals.terms.draft') }}
      </Badge>
      <Button
        type="button"
        size="icon"
        variant="ghost"
        :class="HEADER_BUTTON_CLASS"
        :aria-label="minimized ? t('globals.terms.expand') : t('globals.terms.collapse')"
        @click="minimized = !minimized"
      >
        <ChevronUp v-if="minimized" />
        <Minus v-else class="translate-y-1" />
      </Button>
      <Button
        type="button"
        size="icon"
        variant="ghost"
        :class="[HEADER_BUTTON_CLASS, 'max-sm:hidden']"
        :aria-label="expanded ? t('globals.terms.collapse') : t('globals.terms.expand')"
        @click="toggleExpanded"
      >
        <component :is="expanded ? Minimize2 : Maximize2" />
      </Button>
      <Button
        type="button"
        size="icon"
        variant="ghost"
        :class="HEADER_BUTTON_CLASS"
        :aria-label="t('globals.messages.close')"
        @click="discard"
      >
        <X />
      </Button>
    </header>

    <div v-show="!minimized" class="flex flex-col flex-1 min-h-0">
      <Tabs v-if="showChannelTabs" v-model="channel" class="flex flex-col flex-1 min-h-0">
        <TabsList class="w-max mx-3 mt-3 mb-2">
          <TabsTrigger value="email">{{ $t('globals.terms.email') }}</TabsTrigger>
          <TabsTrigger value="whatsapp">{{ $t('globals.terms.whatsapp') }}</TabsTrigger>
        </TabsList>
        <EmailConversationForm
          ref="emailFormRef"
          v-show="channel === 'email'"
          :initial-contact="props.initialContact"
          @close="dialogOpen = false"
        />
        <WhatsAppConversationForm
          ref="whatsappFormRef"
          v-show="channel === 'whatsapp'"
          :initial-contact="props.initialContact"
          @close="dialogOpen = false"
        />
      </Tabs>

      <EmailConversationForm
        ref="emailFormRef"
        v-else
        :initial-contact="props.initialContact"
        @close="dialogOpen = false"
      />
    </div>
  </section>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronUp, Maximize2, Minimize2, Minus, X } from 'lucide-vue-next'
import { Button } from '@shared-ui/components/ui/button'
import { Badge } from '@shared-ui/components/ui/badge'
import { Tabs, TabsList, TabsTrigger } from '@shared-ui/components/ui/tabs'
import { MACRO_CONTEXT } from '@main/constants/conversation'
import { useCommandPalette } from '@main/features/command/useCommandPalette'
import { useInboxStore } from '@main/stores/inbox'
import EmailConversationForm from './EmailConversationForm.vue'
import WhatsAppConversationForm from './WhatsAppConversationForm.vue'
import { useNewConversationDraft, clearNewConversationDrafts } from './useNewConversationDraft.js'

const HEADER_BUTTON_CLASS = 'h-8 w-8 max-sm:h-11 max-sm:w-11 text-muted-foreground'

const dialogOpen = defineModel({
  required: false,
  default: () => false
})
const props = defineProps({
  initialContact: { type: Object, default: null },
  startMinimized: { type: Boolean, default: false }
})

const { t } = useI18n()
const inboxStore = useInboxStore()
const palette = useCommandPalette()
const drafts = { email: useNewConversationDraft('email'), whatsapp: useNewConversationDraft('whatsapp') }
const channel = ref(!drafts.email.value && drafts.whatsapp.value ? 'whatsapp' : 'email')
const minimized = ref(props.startMinimized)
const expanded = ref(false)

const emailFormRef = ref(null)
const whatsappFormRef = ref(null)

const hasDraft = computed(() => !!drafts[channel.value].value)

const restore = () => {
  minimized.value = false
}

defineExpose({ restore })

const discard = () => {
  clearNewConversationDrafts(window.localStorage)
  dialogOpen.value = false
}

const showChannelTabs = computed(() => inboxStore.whatsappOptions.length > 0)

watch(
  () => dialogOpen.value && !minimized.value && (!showChannelTabs.value || channel.value === 'email'),
  (active) => palette.setMacroContext(active ? MACRO_CONTEXT.NEW_CONVERSATION : MACRO_CONTEXT.REPLY),
  { immediate: true }
)

onUnmounted(() => palette.setMacroContext(MACRO_CONTEXT.REPLY))

const panelClass = computed(() => {
  const base =
    'fixed z-50 flex flex-col overflow-hidden border border-border bg-background shadow-2xl animate-in fade-in-0 duration-200 motion-reduce:animate-none'
  // Minimized: docked bar bottom-right, so a draft can wait while you work
  if (minimized.value) return [base, 'slide-in-from-bottom-4 bottom-0 inset-x-0 sm:inset-x-auto sm:right-4 sm:w-80 sm:rounded-t-lg']
  if (expanded.value) return [base, 'zoom-in-95 inset-4 mx-auto max-w-5xl rounded-xl']
  // KD: open as a centered modal (full screen on phones)
  return [
    base,
    'zoom-in-95 inset-0 sm:inset-auto sm:left-1/2 sm:top-1/2 sm:-translate-x-1/2 sm:-translate-y-1/2 sm:w-[min(44rem,calc(100vw-2rem))] sm:h-[min(42rem,calc(100vh-4rem))] sm:rounded-xl'
  ]
})

const toggleExpanded = () => {
  expanded.value = !expanded.value
  minimized.value = false
}

const focusActiveForm = () => {
  if (!dialogOpen.value || minimized.value) return
  const form = channel.value === 'whatsapp' ? whatsappFormRef.value : emailFormRef.value
  form?.focus()
}

watch([dialogOpen, minimized, channel], focusActiveForm, { flush: 'post' })

// Escape tucks the modal away (draft kept) - unless a dropdown inside it is open
const onKeydown = (e) => {
  if (e.key !== 'Escape' || e.defaultPrevented || !dialogOpen.value || minimized.value) return
  if (document.querySelector('[role="listbox"], [role="menu"], [data-radix-popper-content-wrapper]')) return
  minimized.value = true
}

onMounted(() => {
  inboxStore.fetchInboxes()
  focusActiveForm()
  window.addEventListener('keydown', onKeydown)
})
onUnmounted(() => window.removeEventListener('keydown', onKeydown))
</script>
