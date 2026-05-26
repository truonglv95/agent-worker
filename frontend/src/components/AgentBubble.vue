<script setup lang="ts">
import { computed } from 'vue'
import type { DebateMessage } from '@/stores/debate'

const props = defineProps<{
  message: DebateMessage
}>()

const AGENT_CONFIG: Record<string, { emoji: string; color: string; label: string }> = {
  coordinator: { emoji: '🤖', color: 'var(--cyan)',    label: 'Coordinator' },
  po:          { emoji: '📋', color: 'var(--amber)',   label: 'Product Owner' },
  techlead:    { emoji: '📐', color: 'var(--violet)',  label: 'Tech Lead' },
  qa:          { emoji: '🛡️', color: 'var(--emerald)', label: 'QA Engineer' },
  dev:         { emoji: '💻', color: 'var(--rose)',    label: 'Developer' },
  user:        { emoji: '🧑‍💻', color: 'var(--cyan)',    label: 'You' }
}

const isUser = computed(() => props.message.agent_id?.toLowerCase() === 'user' || props.message.role?.toLowerCase() === 'client')

const agentKey = computed(() => {
  if (isUser.value) return 'user'
  const id = props.message.agent_id?.toLowerCase() ?? ''
  for (const key of Object.keys(AGENT_CONFIG)) {
    if (id.includes(key)) return key
  }
  return 'coordinator'
})

const config = computed(() => AGENT_CONFIG[agentKey.value])

const bubbleClass = computed(() => `debate-bubble--${agentKey.value}`)

function formatTime(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}
</script>

<template>
  <div class="debate-bubble" :class="bubbleClass" :style="isUser ? { alignSelf: 'flex-end', backgroundColor: 'var(--bg-elevated)', border: '1px solid var(--cyan)' } : {}">
    <div class="debate-bubble__avatar">
      {{ config.emoji }}
    </div>

    <div class="debate-bubble__body">
      <div class="debate-bubble__header" :style="isUser ? { flexDirection: 'row-reverse' } : {}">
        <span class="debate-bubble__name" :style="{ color: config.color }">
          {{ message.agent_name || config.label }}
        </span>
        <span v-if="!isUser" class="debate-bubble__role">{{ message.role || config.label }}</span>
        <span v-if="!isUser" class="round-badge">Round {{ message.round }}</span>
      </div>

      <div class="debate-bubble__message">{{ message.message }}</div>

      <div class="debate-bubble__timestamp">{{ formatTime(message.created_at) }}</div>
    </div>
  </div>
</template>
