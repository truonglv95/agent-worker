<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import WorkflowSidebar from '@/components/WorkflowSidebar.vue'
import NewWorkflowModal from '@/components/NewWorkflowModal.vue'
import { useWorkflowStore } from '@/stores/workflow'

const router = useRouter()
const workflowStore = useWorkflowStore()

const showModal = ref(false)
const creating = ref(false)

onMounted(() => {
  void workflowStore.fetchWorkflows()
})

function openModal() {
  showModal.value = true
}

function closeModal() {
  showModal.value = false
}

async function handleWorkflowSubmit(request: string, rounds: number) {
  creating.value = true
  try {
    const id = await workflowStore.createWorkflow(request, rounds)
    closeModal()
    void router.push({ name: 'workflow', params: { id } })
  } catch (err) {
    console.error('[App] createWorkflow error:', err)
  } finally {
    creating.value = false
  }
}
</script>

<template>
  <div class="app-shell">
    <!-- Top bar -->
    <header class="app-header">
      <div class="app-header__logo" @click="router.push('/')" style="cursor: pointer;">
        <div class="app-header__logo-icon">🧠</div>
        <span>AI Agents</span>
      </div>
      <nav class="app-header__nav" style="margin-left: 32px; display: flex; gap: 16px;">
        <router-link to="/" class="nav-link" active-class="nav-link--active">Workflows</router-link>
        <router-link to="/office" class="nav-link" active-class="nav-link--active">Virtual Office</router-link>
      </nav>
      <div class="app-header__spacer" />
      <div class="app-header__status">
        <span class="sse-dot" style="background: var(--emerald); box-shadow: 0 0 6px var(--emerald); animation: pulse-dot 2s ease-in-out infinite;" />
        <span>5 Agents Online</span>
      </div>
      <button class="btn btn-primary btn-sm" @click="openModal">
        ＋ New Project
      </button>
    </header>

    <!-- Body layout -->
    <div class="layout">
      <!-- Left sidebar -->
      <WorkflowSidebar @new-project="openModal" />

      <!-- Main router view (center + inner right panel handled by WorkflowView) -->
      <main class="layout-center" style="grid-column: 2 / -1;">
        <router-view />
      </main>
    </div>

    <!-- Modal -->
    <NewWorkflowModal
      v-if="showModal"
      @submit="handleWorkflowSubmit"
      @close="closeModal"
    />
  </div>
</template>
