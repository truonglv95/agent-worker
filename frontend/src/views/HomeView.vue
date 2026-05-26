<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useWorkflowStore } from '@/stores/workflow'
import { useAgentStore } from '@/stores/agent'

const emit = defineEmits<{
  (e: 'open-modal'): void
}>()

const router = useRouter()
const workflowStore = useWorkflowStore()
const agentStore = useAgentStore()

onMounted(() => {
  if (agentStore.agents.length === 0) agentStore.fetchAgents()
})

const recentWorkflows = computed(() => workflowStore.sortedWorkflows.slice(0, 5))

const showDeleteModal = ref(false)
const workflowToDelete = ref<number | null>(null)

function confirmDelete(id: number, event: Event) {
  event.stopPropagation()
  workflowToDelete.value = id
  showDeleteModal.value = true
}

async function executeDelete() {
  if (workflowToDelete.value !== null) {
    await workflowStore.deleteWorkflow(workflowToDelete.value)
    showDeleteModal.value = false
    workflowToDelete.value = null
  }
}

function cancelDelete() {
  showDeleteModal.value = false
  workflowToDelete.value = null
}

const ROLE_EMOJI: Record<string, string> = {
  coordinator: '🤖',
  'product owner': '📋',
  po: '📋',
  'tech lead': '📐',
  techlead: '📐',
  qa: '🛡️',
  quality: '🛡️',
  frontend: '🎨',
  backend: '⚙️',
  developer: '💻',
  dev: '💻',
}

function agentEmoji(role: string): string {
  const r = role.toLowerCase()
  for (const [key, emoji] of Object.entries(ROLE_EMOJI)) {
    if (r.includes(key)) return emoji
  }
  return '🧑‍💻'
}

function statusClass(status: string): string {
  const map: Record<string, string> = {
    pending: 'badge--pending',
    processing: 'badge--processing',
    completed: 'badge--completed',
    failed: 'badge--failed'
  }
  return map[status] ?? 'badge--pending'
}

function navigate(id: number) {
  void router.push({ name: 'workflow', params: { id } })
}
</script>

<template>
  <div class="home-view">
    <!-- Hero icon -->
    <div class="home-view__hero-icon">🧠</div>

    <!-- Heading -->
    <div>
      <h1 class="home-view__title">Multi-Agent Dev Workspace</h1>
      <p class="home-view__subtitle" style="margin: 12px auto 0;">
        A team of 5 specialized AI agents collaborates through structured debates to plan,
        architect, and deliver your software project from idea to implementation.
      </p>
    </div>

    <div class="agent-grid">
      <div v-if="agentStore.loading" class="agent-pill" style="opacity:0.5">⏳ Đang tải...</div>
      <div
        v-for="agent in agentStore.agents"
        :key="agent.id"
        class="agent-pill"
        :class="{ 'agent-pill--inactive': agent.active === false }"
      >
        <span>{{ agentEmoji(agent.role) }}</span>
        <span style="font-weight: 600; color: var(--text-primary);">{{ agent.name }}</span>
        <span style="color: var(--text-muted); font-size: 12px;">— {{ agent.role }}</span>
        <span v-if="agent.active === false" style="color: var(--text-muted); font-size: 10px; margin-left: auto;">(On Bench)</span>
      </div>
    </div>

    <!-- CTA button -->
    <button class="btn btn-primary" style="padding: 12px 32px; font-size: 16px;" @click="emit('open-modal')">
      ⚡ Start Your First Project
    </button>

    <!-- Recent workflows -->
    <div v-if="recentWorkflows.length > 0" style="width: 100%; max-width: 600px;">
      <div
        style="font-size: 12px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.5px; color: var(--text-muted); margin-bottom: 12px;"
      >
        Recent Projects
      </div>
      <div style="display: flex; flex-direction: column; gap: 8px;">
        <div
          v-for="wf in recentWorkflows"
          :key="wf.id"
          class="glass-card"
          style="padding: 12px 16px; cursor: pointer; display: flex; align-items: center; gap: 12px;"
          @click="navigate(wf.id)"
        >
          <div style="flex: 1; min-width: 0;">
            <div class="truncate" style="font-size: 14px; font-weight: 500; color: var(--text-primary);">
              {{ wf.request }}
            </div>
            <div style="font-size: 11px; color: var(--text-muted); margin-top: 3px;">
              {{ wf.debate_rounds }} debate round{{ wf.debate_rounds !== 1 ? 's' : '' }}
            </div>
          </div>
          <span class="badge" :class="statusClass(wf.status)">{{ wf.status }}</span>
          <button class="btn btn-danger" style="padding: 4px 8px; font-size: 12px; margin-left: 8px;" @click.stop="confirmDelete(wf.id, $event)">🗑️</button>
          <span style="color: var(--text-muted); font-size: 18px; margin-left: 8px;">→</span>
        </div>
      </div>
    </div>

    <!-- Delete Confirmation Modal -->
    <div v-if="showDeleteModal" class="modal-backdrop">
      <div class="modal-content glass-card">
        <h2>Delete Project?</h2>
        <p style="margin-top: 12px; margin-bottom: 24px; color: var(--text-muted);">
          Are you sure you want to delete this project? This will permanently delete all conversations, tasks, and data.
        </p>
        <div style="display: flex; gap: 12px; justify-content: flex-end;">
          <button class="btn btn-secondary" @click="cancelDelete">Cancel</button>
          <button class="btn btn-danger" @click="executeDelete">Delete</button>
        </div>
      </div>
    </div>
  </div>
</template>
