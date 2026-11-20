<!--
  KD: Zendesk-style left panel on the ticket page - requester, assignee, group,
  priority, tags and the ticket's fields (custom attributes), stacked as labelled
  fields like Zendesk's ticket properties.
-->
<template>
  <div v-if="conv" class="flex h-full min-h-0 flex-col overflow-y-auto overscroll-contain">
    <div class="space-y-4 border-b border-border p-4">
      <div>
        <p :class="LABEL">Requester</p>
        <p class="truncate text-sm font-medium">{{ requesterName }}</p>
        <p v-if="conv.contact?.email" class="truncate text-xs text-muted-foreground">{{ conv.contact.email }}</p>
      </div>

      <div>
        <p :class="LABEL">{{ t('globals.terms.assignee') }}</p>
        <SelectAgentCombobox
          v-model="conv.assigned_user_id"
          include-none
          current-user-first
          @select="selectAgent"
        />
        <button
          v-if="String(conv.assigned_user_id) !== String(userStore.userID)"
          type="button"
          class="mt-1 text-xs text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
          @click="takeIt"
        >
          take it
        </button>
      </div>

      <div>
        <p :class="LABEL">Group</p>
        <SelectTeamCombobox :model-value="conv.assigned_team_id" include-none @select="selectTeam" />
      </div>

      <div>
        <p :class="LABEL">{{ t('globals.terms.priority', 1) }}</p>
        <SelectComboBox
          v-model="conv.priority_id"
          :items="conversationStore.priorityOptions"
          :placeholder="t('placeholders.selectPriority')"
          type="priority"
          @select="selectPriority"
        />
      </div>

      <div>
        <p :class="LABEL">Tags</p>
        <SelectTagCombobox
          multiple
          compact
          keep-open-on-select
          :model-value="conv.tags || []"
          @update:modelValue="onTagsChange"
        />
      </div>
    </div>

    <!-- Ticket info + KD ticket fields (custom attributes) -->
    <div class="p-4">
      <p :class="LABEL" class="mb-2">Ticket fields</p>
      <ConversationInfo />
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useConversationStore } from '@/stores/conversation'
import { useUserStore } from '@/stores/user'
import { TAG_ACTION } from '@/constants/conversation'
import ConversationInfo from './ConversationInfo.vue'
import SelectComboBox from '@main/components/combobox/SelectCombobox.vue'
import SelectAgentCombobox from '@main/components/combobox/SelectAgentCombobox.vue'
import SelectTeamCombobox from '@main/components/combobox/SelectTeamCombobox.vue'
import SelectTagCombobox from '@main/components/combobox/SelectTagCombobox.vue'

const LABEL = 'mb-1 text-xs font-medium text-muted-foreground'

const { t } = useI18n()
const conversationStore = useConversationStore()
const userStore = useUserStore()
const conv = computed(() => conversationStore.current)
const requesterName = computed(() => {
  const c = conv.value?.contact
  return c ? [c.first_name, c.last_name].filter(Boolean).join(' ') : ''
})

function selectAgent (agent) {
  if (agent.value === 'none') return conversationStore.removeAssignee('user')
  conv.value.assigned_user_id = agent.value
  conversationStore.updateAssignee('user', { assignee_id: parseInt(agent.value) })
}

// Zendesk's "take it": assign the ticket to yourself in one click
function takeIt () {
  selectAgent({ value: String(userStore.userID) })
}

function selectTeam (team) {
  if (team.value === 'none') return conversationStore.removeAssignee('team')
  conversationStore.updateAssignee('team', { assignee_id: parseInt(team.value) })
}

function selectPriority (priority) {
  conv.value.priority = priority.label
  conv.value.priority_id = priority.value
  conversationStore.updatePriority(priority.label)
}

function onTagsChange (newTags) {
  const current = conv.value?.tags || []
  if (newTags.length === current.length && newTags.every((tag) => current.includes(tag))) return
  conversationStore.updateConversationTags(conv.value.uuid, TAG_ACTION.SET, newTags)
}
</script>
