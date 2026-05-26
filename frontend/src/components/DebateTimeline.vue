<script setup lang="ts">
import { ref, watch, nextTick, computed } from 'vue'
import AgentBubble from './AgentBubble.vue'
import type { DebateMessage } from '@/stores/debate'
import { useDebateStore } from '@/stores/debate'

const props = defineProps<{
  debates: DebateMessage[]
  loading: boolean
  sseConnected: boolean
  workflowStatus: string
}>()

const debateStore = useDebateStore()

const timelineRef = ref<HTMLElement | null>(null)
const bottomRef = ref<HTMLElement | null>(null)

// Auto-scroll to bottom whenever debates change
watch(
  () => props.debates.length,
  async () => {
    await nextTick()
    bottomRef.value?.scrollIntoView({ behavior: 'smooth' })
  }
)

// Group debates by round for separator rendering
const groupedByRound = computed(() => {
  const groups = new Map<number, DebateMessage[]>()
  for (const msg of props.debates) {
    if (!groups.has(msg.round)) groups.set(msg.round, [])
    groups.get(msg.round)!.push(msg)
  }
  return [...groups.entries()].sort((a, b) => a[0] - b[0])
})

const thinkingAgent = computed(() => debateStore.thinkingAgent)

const showTyping = computed(
  () => props.sseConnected && props.workflowStatus === 'processing' && !thinkingAgent.value
)

const sseLabelText = computed(() =>
  props.sseConnected ? 'Live' : 'Disconnected'
)
</script>

<template>
  <div style="display: flex; flex-direction: column; height: 100%; overflow: hidden;">
    <!-- Status bar -->
    <div style="padding: 8px 16px; border-bottom: 1px solid var(--border); display: flex; align-items: center; justify-content: space-between; flex-shrink: 0;">
      <span class="text-xs text-muted">{{ debates.length }} message{{ debates.length !== 1 ? 's' : '' }}</span>
      <div class="sse-indicator">
        <span class="sse-dot" :class="{ 'sse-dot--live': sseConnected }" />
        <span>{{ sseLabelText }}</span>
      </div>
    </div>

    <!-- Timeline -->
    <div ref="timelineRef" class="debate-timeline">
      <!-- Loading state -->
      <div v-if="loading" class="empty-state">
        <div class="typing-dots"><span /><span /><span /></div>
        <p class="text-sm text-muted">Loading debate history…</p>
      </div>

      <!-- Empty state -->
      <div v-else-if="debates.length === 0 && !sseConnected" class="empty-state">
        <div class="empty-state__icon">💬</div>
        <p class="empty-state__title">No debate messages yet</p>
        <p class="empty-state__desc">Agent discussions will appear here in real-time as the workflow runs.</p>
      </div>

      <!-- Waiting for first message -->
      <div v-else-if="debates.length === 0 && sseConnected" class="empty-state">
        <div class="typing-dots"><span /><span /><span /></div>
        <p class="text-sm text-muted">
          <template v-if="thinkingAgent">
            {{ thinkingAgent.agent_name }} is thinking…
          </template>
          <template v-else>
            Agents are warming up…
          </template>
        </p>
      </div>

      <!-- Grouped messages with round separators -->
      <template v-for="[round, messages] in groupedByRound" :key="round">
        <div v-if="round > 1" class="round-separator">
          Round {{ round - 1 }} Complete
        </div>
        <AgentBubble
          v-for="msg in messages"
          :key="msg.id"
          :message="msg"
        />
      </template>

      <!-- Typing indicator (generic) -->
      <div v-if="showTyping" class="typing-indicator">
        <div class="typing-dots"><span /><span /><span /></div>
        <span>Agents are debating…</span>
      </div>

      <!-- Thinking indicator (specific agent) -->
      <div v-else-if="thinkingAgent && debates.length > 0" class="typing-indicator">
        <div class="typing-dots"><span /><span /><span /></div>
        <span>{{ thinkingAgent.agent_name }} is thinking…</span>
      </div>

      <!-- Bottom anchor for auto-scroll -->
      <div ref="bottomRef" style="height: 1px; flex-shrink: 0;" />
    </div>
  </div>
</template>
