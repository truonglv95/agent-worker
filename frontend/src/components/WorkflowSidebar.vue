<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useWorkflowStore } from '@/stores/workflow'
import ConfirmDeleteModal from '@/components/ConfirmDeleteModal.vue'

const emit = defineEmits<{
  (e: 'new-project'): void
}>()

const router = useRouter()
const route = useRoute()
const workflowStore = useWorkflowStore()

const workflows = computed(() => workflowStore.sortedWorkflows)

const activeId = computed(() => {
  if (route.name === 'workflow') {
    return Number(route.params.id)
  }
  return null
})

function navigate(id: number) {
  void router.push({ name: 'workflow', params: { id } })
}

function formatDate(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

function statusClass(status: string): string {
  const map: Record<string, string> = {
    pending:    'badge--pending',
    processing: 'badge--processing',
    completed:  'badge--completed',
    failed:     'badge--failed'
  }
  return map[status] ?? 'badge--pending'
}

const workflowToDelete = ref<number | null>(null)

function promptDelete(id: number) {
  workflowToDelete.value = id
}

async function doDelete(id: number) {
  try {
    await workflowStore.deleteWorkflow(id)
    if (activeId.value === id) {
      void router.push('/')
    }
    workflowToDelete.value = null
  } catch (e) {
    alert('Failed to delete project: ' + e)
    workflowToDelete.value = null
  }
}
</script>

<template>
  <aside class="sidebar">
    <div class="sidebar__header">
      <button class="btn btn-primary w-full" @click="emit('new-project')">
        <span>＋</span>
        New Project
      </button>
    </div>

    <nav class="sidebar__list">
      <div v-if="workflowStore.loading && workflows.length === 0" class="empty-state">
        <div class="typing-dots">
          <span /><span /><span />
        </div>
        <p class="text-sm text-muted">Loading…</p>
      </div>

      <div v-else-if="workflows.length === 0" class="empty-state">
        <div class="empty-state__icon">🚀</div>
        <p class="empty-state__title">No projects yet</p>
        <p class="empty-state__desc">Create your first multi-agent workflow above</p>
      </div>

      <div
        v-for="wf in workflows"
        :key="wf.id"
        class="sidebar-item"
        :class="{ 'sidebar-item--active': activeId === wf.id }"
        @click="navigate(wf.id)"
        style="display: flex; align-items: center; justify-content: space-between;"
      >
        <div style="flex: 1; min-width: 0;">
          <div class="sidebar-item__request">{{ wf.request }}</div>
          <div class="sidebar-item__meta">
            <span class="badge" :class="statusClass(wf.status)">{{ wf.status }}</span>
            <span class="sidebar-item__date">{{ formatDate(wf.created_at) }}</span>
          </div>
        </div>
        <button 
          class="btn btn-ghost btn-sm" 
          style="padding: 4px; color: var(--rose); margin-left: 8px; flex-shrink: 0;"
          title="Delete Project"
          @click.stop="promptDelete(wf.id)"
        >
          🗑
        </button>
      </div>
    </nav>

    <ConfirmDeleteModal 
      v-if="workflowToDelete !== null"
      :workflow-id="workflowToDelete"
      @confirm="doDelete"
      @cancel="workflowToDelete = null"
    />
  </aside>
</template>
