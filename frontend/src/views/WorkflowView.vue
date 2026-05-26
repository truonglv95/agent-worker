<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import DebateTimeline from '@/components/DebateTimeline.vue'
import TaskBoard from '@/components/TaskBoard.vue'
import SystemLogsDock from '@/components/SystemLogsDock.vue'
import ConfirmDeleteModal from '@/components/ConfirmDeleteModal.vue'
import { useDebateStore } from '@/stores/debate'
import { useWorkflowStore } from '@/stores/workflow'
import { useSSE } from '@/composables/useSSE'

const props = defineProps<{ id: number }>()

const debateStore = useDebateStore()
const workflowStore = useWorkflowStore()
const router = useRouter()

const loading = ref(false)
const loadError = ref('')
const syncing = ref(false)
const syncLinks = ref<{name: string, url: string}[]>([])

const followupText = ref('')
const sendingFollowup = ref(false)

const canFollowup = computed(() => {
  return workflowStatus.value === 'completed' || workflowStatus.value === 'synced' || workflowStatus.value === 'failed'
})

async function submitFollowup() {
  if (!workflowId.value || !followupText.value.trim() || !canFollowup.value) return
  sendingFollowup.value = true
  try {
    await workflowStore.sendFollowup(workflowId.value, followupText.value.trim())
    followupText.value = ''
  } catch (e) {
    alert('Failed to send followup: ' + e)
  } finally {
    sendingFollowup.value = false
  }
}

async function doSync() {
  if (!workflowId.value) return
  syncing.value = true
  try {
    const links = await workflowStore.syncWorkflow(workflowId.value)
    syncLinks.value = links
    debateStore.updateActiveStatus('synced')
  } catch (e) {
    alert('Failed to sync: ' + e)
  } finally {
    syncing.value = false
  }
}

const resuming = ref(false)
async function doResume() {
  if (!workflowId.value) return
  resuming.value = true
  try {
    await debateStore.resumeWorkflow(workflowId.value)
  } catch (e) {
    alert('Failed to resume: ' + e)
  } finally {
    resuming.value = false
  }
}

const showDeleteModal = ref(false)

function promptDelete() {
  showDeleteModal.value = true
}

async function doDelete() {
  if (!workflowId.value) return
  try {
    await workflowStore.deleteWorkflow(workflowId.value)
    void router.push('/')
  } catch (e) {
    alert('Failed to delete project: ' + e)
    showDeleteModal.value = false
  }
}

// Reactive workflowId ref for SSE composable
const workflowId = ref<number | null>(null)

// Initialize SSE composable — watches workflowId
useSSE(workflowId)

async function loadData(id: number) {
  loading.value = true
  loadError.value = ''
  debateStore.clearActive()
  workflowId.value = null

  try {
    await debateStore.loadWorkflow(id)
    // Activate SSE after data loaded
    workflowId.value = id
  } catch {
    loadError.value = 'Failed to load workflow. Please try again.'
  } finally {
    loading.value = false
  }
}

// Load on mount and when id changes
onMounted(() => {
  void loadData(props.id)
})

watch(
  () => props.id,
  (newId) => {
    void loadData(newId)
  }
)

onUnmounted(() => {
  workflowId.value = null
  debateStore.clearActive()
})

const active = computed(() => debateStore.active)
const debates = computed(() => debateStore.debates)
const tasks = computed(() => debateStore.tasks)
const workflowStatus = computed(() => debateStore.workflowStatus)
const sseConnected = computed(() => debateStore.sseConnected)
const completedCount = computed(() => debateStore.completedTaskCount)
const progressPercent = computed(() => debateStore.taskCompletionPercent)

function statusClass(status: string): string {
  const map: Record<string, string> = {
    pending:    'badge--pending',
    processing: 'badge--processing',
    completed:  'badge--completed',
    failed:     'badge--failed',
    paused:     'badge--pending'
  }
  return map[status] ?? 'badge--pending'
}
</script>

<template>
  <div style="display: flex; flex-direction: column; height: 100%; overflow: hidden;">
    <!-- Error state -->
    <div v-if="loadError" class="empty-state" style="padding: 60px 40px;">
      <div class="empty-state__icon">⚠️</div>
      <p class="empty-state__title">{{ loadError }}</p>
      <button class="btn btn-primary" @click="loadData(props.id)">Retry</button>
    </div>

    <!-- Loading skeleton -->
    <div v-else-if="loading && !active" class="empty-state" style="height: 100%;">
      <div class="typing-dots"><span /><span /><span /></div>
      <p class="text-sm text-muted">Loading workflow…</p>
    </div>

    <!-- Workflow content -->
    <template v-else-if="active">
      <!-- Workflow header -->
      <div class="workflow-header">
        <div class="workflow-header__title">{{ active.request }}</div>
        <div class="workflow-header__meta">
          <span class="badge" :class="statusClass(workflowStatus)">{{ workflowStatus }}</span>
          <span class="text-xs text-muted">{{ active.debate_rounds }} round{{ active.debate_rounds !== 1 ? 's' : '' }}</span>
          <span v-if="tasks.length > 0" class="text-xs text-muted">
            · {{ completedCount }}/{{ tasks.length }} tasks · {{ progressPercent }}%
          </span>
          <div style="margin-left: auto; display: flex; gap: 8px;">
            <button 
              v-if="workflowStatus === 'failed' || workflowStatus === 'paused' || workflowStatus === 'synced'" 
              class="btn btn-primary" 
              style="padding: 4px 12px; font-size: 12px; background: var(--amber); border-color: var(--amber); color: #000; font-weight: 600;"
              :disabled="resuming"
              @click="doResume"
            >
              {{ resuming ? 'Resuming...' : '▶ Resume Tasks' }}
            </button>
            <button 
              class="btn btn-danger" 
              style="padding: 4px 12px; font-size: 12px;"
              @click="promptDelete"
            >
              Delete Project
            </button>
            <button 
              v-if="workflowStatus === 'completed' || workflowStatus === 'synced'" 
              class="btn btn-secondary" 
              style="padding: 4px 12px; font-size: 12px;"
              :disabled="syncing"
              @click="doSync"
            >
              {{ syncing ? 'Syncing...' : (workflowStatus === 'synced' ? 'Sync Again' : 'Sync Platforms') }}
            </button>
          </div>
          <template v-if="syncLinks.length > 0">
            <a v-for="link in syncLinks" :key="link.name" :href="link.url" target="_blank" style="font-size: 12px; color: var(--cyan); margin-left: 8px;">
              {{ link.name }} ↗
            </a>
          </template>
        </div>
        <div v-if="tasks.length > 0" class="progress-bar" style="margin-top: 10px;">
          <div class="progress-bar__fill" :style="{ width: progressPercent + '%' }" />
        </div>
      </div>

      <!-- Two-panel body -->
      <div style="display: flex; flex: 1; overflow: hidden;">
        <!-- Center: Debate Timeline -->
        <div style="flex: 1; overflow: hidden; display: flex; flex-direction: column;">
          <div style="flex: 1; overflow: hidden;">
            <DebateTimeline
              :debates="debates"
              :loading="loading"
              :sse-connected="sseConnected"
              :workflow-status="workflowStatus"
            />
          </div>
          <!-- Followup Chat Box -->
          <div style="padding: 12px 16px; border-top: 1px solid var(--border); display: flex; gap: 8px; align-items: center; background: var(--bg);">
            <input 
              v-model="followupText" 
              type="text" 
              class="form-input" 
              placeholder="Nhập yêu cầu bổ sung cho dự án này..." 
              style="flex: 1;"
              :disabled="!canFollowup || sendingFollowup"
              @keyup.enter="submitFollowup"
            />
            <button 
              class="btn btn-primary" 
              :disabled="!canFollowup || sendingFollowup || !followupText.trim()"
              @click="submitFollowup"
            >
              {{ sendingFollowup ? 'Đang gửi...' : 'Gửi' }}
            </button>
          </div>
        </div>

        <!-- Right: Task Board -->
        <div style="width: var(--panel-width); border-left: 1px solid var(--border); overflow: hidden; flex-shrink: 0;">
          <TaskBoard :tasks="tasks" :workflow-status="workflowStatus" />
        </div>
      </div>
    </template>

    <SystemLogsDock v-if="active" />
    <ConfirmDeleteModal 
      v-if="showDeleteModal && workflowId"
      :workflow-id="workflowId"
      @confirm="doDelete"
      @cancel="showDeleteModal = false"
    />
  </div>
</template>
