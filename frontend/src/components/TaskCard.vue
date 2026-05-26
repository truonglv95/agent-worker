<script setup lang="ts">
import { computed } from 'vue'
import type { Task } from '@/stores/debate'

const props = defineProps<{
  task: Task
}>()

const emit = defineEmits<{
  (e: 'toggle-content', id: number): void
}>()

const typeClass = computed(() => {
  const t = props.task.type?.toLowerCase() ?? ''
  return `badge--${t.replace('_', '')}`
})

const statusClass = computed(() => {
  const map: Record<string, string> = {
    pending:    'badge--pending',
    processing: 'badge--processing',
    completed:  'badge--completed',
    failed:     'badge--failed'
  }
  return map[props.task.status] ?? 'badge--pending'
})

const agentEmoji = computed(() => {
  const a = props.task.assigned_to?.toLowerCase() ?? ''
  if (a.includes('coordinator')) return '🤖'
  if (a.includes('po'))          return '📋'
  if (a.includes('techlead'))    return '📐'
  if (a.includes('qa'))          return '🛡️'
  if (a.includes('dev'))         return '💻'
  return '🤖'
})

const hasContent = computed(() => !!props.task.content?.trim())
</script>

<template>
  <div class="task-card">
    <div class="task-card__header">
      <div class="task-card__title">{{ task.title }}</div>
      <span class="badge" :class="typeClass">{{ task.type }}</span>
    </div>

    <div class="task-card__meta">
      <span class="badge" :class="statusClass">{{ task.status }}</span>
      <span class="agent-chip">
        {{ agentEmoji }} {{ task.assigned_to }}
      </span>
    </div>

    <p v-if="task.description" class="task-card__description">{{ task.description }}</p>

    <div v-if="hasContent" style="margin-top: 10px;">
      <button
        class="btn btn-ghost btn-sm"
        @click="emit('toggle-content', task.id)"
      >
        {{ task.showContent ? '▲ Hide Content' : '▼ View Content' }}
      </button>

      <div v-if="task.showContent" class="code-block">{{ task.content }}</div>
    </div>
  </div>
</template>
