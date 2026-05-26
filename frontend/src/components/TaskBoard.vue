<script setup lang="ts">
import { computed } from 'vue'
import TaskCard from './TaskCard.vue'
import { useDebateStore } from '@/stores/debate'
import type { Task } from '@/stores/debate'

const props = defineProps<{
  tasks: Task[]
  workflowStatus: string
}>()

const debateStore = useDebateStore()

const completedCount = computed(() => props.tasks.filter((t) => t.status === 'completed').length)
const totalCount = computed(() => props.tasks.length)
const progressPercent = computed(() => {
  if (totalCount.value === 0) return 0
  return Math.round((completedCount.value / totalCount.value) * 100)
})

function toggleContent(id: number) {
  const task = debateStore.active?.tasks.find((t) => t.id === id)
  if (task) {
    task.showContent = !task.showContent
  }
}
</script>

<template>
  <div class="task-board">
    <!-- Header -->
    <div class="panel-header">
      <span class="panel-header__title">📋 Task Board</span>
      <span class="text-xs text-muted">{{ completedCount }}/{{ totalCount }} done</span>
    </div>

    <!-- Progress bar -->
    <div v-if="totalCount > 0" style="padding: 10px 16px 0; flex-shrink: 0;">
      <div class="progress-bar">
        <div class="progress-bar__fill" :style="{ width: progressPercent + '%' }" />
      </div>
      <div style="margin-top: 4px; text-align: right; font-size: 11px; color: var(--text-muted);">
        {{ progressPercent }}% complete
      </div>
    </div>

    <!-- Task list -->
    <div class="task-board__body">
      <div v-if="tasks.length === 0" class="empty-state">
        <div class="empty-state__icon">
          <span v-if="workflowStatus === 'processing'">⚙️</span>
          <span v-else>📭</span>
        </div>
        <p class="empty-state__title">
          {{ workflowStatus === 'processing' ? 'Generating tasks…' : 'No tasks yet' }}
        </p>
        <p class="empty-state__desc">
          {{ workflowStatus === 'processing'
            ? 'Tasks will appear as agents complete each phase.'
            : 'Tasks will be created when the workflow runs.'
          }}
        </p>
      </div>

      <TaskCard
        v-for="task in tasks"
        :key="task.id"
        :task="task"
        @toggle-content="toggleContent"
      />
    </div>
  </div>
</template>
