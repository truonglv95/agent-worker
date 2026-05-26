<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import OfficeRoom from '@/components/office/OfficeRoom.vue'
import AgentAvatar from '@/components/office/AgentAvatar.vue'
import OfficeLogPanel from '@/components/office/OfficeLogPanel.vue'
import { useAgentStore, type Agent } from '@/stores/agent'

const agentStore = useAgentStore()

// ─── Room config ──────────────────────────────────────────────────────────────
const ROOM_NAMES: Record<string, string> = {
  meeting: 'Meeting Room',
  rest: 'REST ROOM',
  game: 'GAME ROOM',
  rnd: 'R&D Lab',
  qa_room: 'QA Room',
  po_room: 'Product Room',
  lobby: 'Lobby',
  pantry: 'PANTRY',
}

// Default room assignment based on role keyword
function defaultRoomForRole(role: string): string {
  const r = role.toLowerCase()
  if (r.includes('coordinator')) return 'meeting'
  if (r.includes('product owner') || r.includes('po')) return 'po_room'
  if (r.includes('tech lead') || r.includes('techlead')) return 'rnd'
  if (r.includes('qa') || r.includes('quality')) return 'qa_room'
  if (r.includes('frontend') || r.includes('fe')) return 'rnd'
  if (r.includes('backend') || r.includes('be')) return 'rnd'
  if (r.includes('developer') || r.includes('dev')) return 'rnd'
  return 'lobby'
}

// Extended agent with roomId for local UI state
interface OfficeAgent extends Agent {
  roomId: string
  status?: string
}

const officeAgents = ref<OfficeAgent[]>([])

const logs = ref<{ time: string; message: string }[]>([])

onMounted(async () => {
  await agentStore.fetchAgents()
  officeAgents.value = agentStore.agents.map((a) => ({
    ...a,
    roomId: a.active ? defaultRoomForRole(a.role) : 'lobby',
    status: a.active ? 'Đang làm việc' : 'On Bench (Chờ việc)'
  }))
})

const activeRoomId = ref('rnd')
const activeRoomName = computed(() => ROOM_NAMES[activeRoomId.value] || activeRoomId.value)
const activeRoomMembers = computed(() =>
  officeAgents.value.filter((a) => a.roomId === activeRoomId.value).map((a) => a.role)
)

function selectRoom(roomId: string) {
  activeRoomId.value = roomId
}

function handleDragStart(agentId: string, event: DragEvent) {
  if (event.dataTransfer) {
    event.dataTransfer.setData('text/plain', agentId)
    event.dataTransfer.effectAllowed = 'move'
  }
}

function handleDropAgent(roomId: string, event: DragEvent) {
  const agentId = event.dataTransfer?.getData('text/plain')
  if (agentId) {
    const agent = officeAgents.value.find((a) => a.id === agentId)
    if (agent) {
      agent.roomId = roomId
      activeRoomId.value = roomId
      const timeStr = new Date().toLocaleTimeString('vi-VN', {
        hour12: false,
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit'
      })
      logs.value.unshift({
        time: timeStr,
        message: `[${agent.role}] vừa di chuyển vào ${ROOM_NAMES[roomId] ?? roomId}`
      })
    }
  }
}

// ─── Manage Agents Popup ──────────────────────────────────────────────────────
const showManagePopup = ref(false)
const editingAgent = ref<Agent | null>(null)
const agentForm = ref({ id: '', name: '', role: '', system_prompt: '', task_description: '', active: true })
const formError = ref('')
const saving = ref(false)

function openCreate() {
  editingAgent.value = null
  agentForm.value = { id: '', name: '', role: '', system_prompt: '', task_description: '', active: true }
  formError.value = ''
}

function openEdit(agent: Agent) {
  editingAgent.value = agent
  agentForm.value = { ...agent, active: agent.active ?? true }
  formError.value = ''
}

async function saveAgent() {
  formError.value = ''
  if (!agentForm.value.name || !agentForm.value.role) {
    formError.value = 'Name và Role là bắt buộc'
    return
  }
  saving.value = true
  try {
    if (editingAgent.value) {
      await agentStore.updateAgent(editingAgent.value.id, agentForm.value)
    } else {
      if (!agentForm.value.id) {
        agentForm.value.id = agentForm.value.name.toLowerCase().replace(/\s+/g, '-')
      }
      await agentStore.createAgent(agentForm.value)
    }
    // Sync office agents
    officeAgents.value = agentStore.agents.map((a) => {
      const existing = officeAgents.value.find((o) => o.id === a.id)
      return { ...a, roomId: existing?.roomId ?? defaultRoomForRole(a.role), status: existing?.status ?? 'Đang làm việc' }
    })
    openCreate()
  } catch (e: unknown) {
    formError.value = (e as Error).message ?? 'Lỗi khi lưu agent'
  } finally {
    saving.value = false
  }
}

async function deleteAgent(id: string) {
  if (!confirm('Xóa agent này?')) return
  await agentStore.deleteAgent(id)
  officeAgents.value = officeAgents.value.filter((a) => a.id !== id)
}
</script>

<template>
  <div class="virtual-office-layout">
    <!-- Main Map Area -->
    <div class="map-container">
      <!-- Toolbar -->
      <div class="office-toolbar">
        <span class="office-title">🏢 Virtual Office</span>
        <button class="btn-manage" @click="showManagePopup = true">⚙️ Quản lý Agents</button>
      </div>

      <div class="office-grid">
        <OfficeRoom name="Meeting Room" color="blue" gridArea="meeting"
          @click="selectRoom('meeting')" @drop-agent="handleDropAgent('meeting', $event)">
          <div class="meeting-table"></div>
          <AgentAvatar v-for="agent in officeAgents.filter(a => a.roomId === 'meeting')"
            :key="agent.id" v-bind="agent" @dragstart="handleDragStart" />
        </OfficeRoom>

        <OfficeRoom name="Product Room" color="yellow" gridArea="po_room"
          @click="selectRoom('po_room')" @drop-agent="handleDropAgent('po_room', $event)">
          <div class="desk"></div>
          <AgentAvatar v-for="agent in officeAgents.filter(a => a.roomId === 'po_room')"
            :key="agent.id" v-bind="agent" @dragstart="handleDragStart" />
        </OfficeRoom>

        <OfficeRoom name="R&D Lab" color="purple" gridArea="rnd"
          @click="selectRoom('rnd')" @drop-agent="handleDropAgent('rnd', $event)">
          <div class="desk"></div>
          <AgentAvatar v-for="agent in officeAgents.filter(a => a.roomId === 'rnd')"
            :key="agent.id" v-bind="agent" @dragstart="handleDragStart" />
        </OfficeRoom>

        <OfficeRoom name="QA Room" color="green" gridArea="qa_room"
          @click="selectRoom('qa_room')" @drop-agent="handleDropAgent('qa_room', $event)">
          <div class="desk"></div>
          <AgentAvatar v-for="agent in officeAgents.filter(a => a.roomId === 'qa_room')"
            :key="agent.id" v-bind="agent" @dragstart="handleDragStart" />
        </OfficeRoom>

        <OfficeRoom name="REST ROOM" color="purple" gridArea="rest"
          @click="selectRoom('rest')" @drop-agent="handleDropAgent('rest', $event)">
          <div class="sofa"></div>
          <AgentAvatar v-for="agent in officeAgents.filter(a => a.roomId === 'rest')"
            :key="agent.id" v-bind="agent" @dragstart="handleDragStart" />
        </OfficeRoom>

        <OfficeRoom name="GAME ROOM" color="green" gridArea="game"
          @click="selectRoom('game')" @drop-agent="handleDropAgent('game', $event)">
          <div class="pool-table"></div>
          <AgentAvatar v-for="agent in officeAgents.filter(a => a.roomId === 'game')"
            :key="agent.id" v-bind="agent" @dragstart="handleDragStart" />
        </OfficeRoom>

        <OfficeRoom name="PANTRY" color="yellow" gridArea="pantry"
          @click="selectRoom('pantry')" @drop-agent="handleDropAgent('pantry', $event)">
          <div class="kitchen-counter"></div>
          <AgentAvatar v-for="agent in officeAgents.filter(a => a.roomId === 'pantry')"
            :key="agent.id" v-bind="agent" @dragstart="handleDragStart" />
        </OfficeRoom>

        <OfficeRoom name="Lobby" color="blue" gridArea="lobby"
          @click="selectRoom('lobby')" @drop-agent="handleDropAgent('lobby', $event)">
          <AgentAvatar v-for="agent in officeAgents.filter(a => a.roomId === 'lobby')"
            :key="agent.id" v-bind="agent" @dragstart="handleDragStart" />
        </OfficeRoom>
      </div>
    </div>

    <!-- Right Sidebar -->
    <OfficeLogPanel
      :roomName="activeRoomName"
      :presentMembers="activeRoomMembers"
      :logs="logs"
    />

    <!-- ─── Manage Agents Popup ─────────────────────────────────────────── -->
    <Teleport to="body">
      <div v-if="showManagePopup" class="popup-overlay" @click.self="showManagePopup = false">
        <div class="popup-panel">
          <div class="popup-header">
            <h2>⚙️ Quản lý Agents</h2>
            <button class="popup-close" @click="showManagePopup = false">✕</button>
          </div>

          <div class="popup-body">
            <!-- Left: list -->
            <div class="agent-list">
              <div
                v-for="agent in agentStore.agents"
                :key="agent.id"
                class="agent-list-item"
                :class="{ active: editingAgent?.id === agent.id }"
                @click="openEdit(agent)"
              >
                <div class="agent-list-avatar" :style="{ background: `hsl(${[...agent.role].reduce((a,c)=>a+c.charCodeAt(0),0) % 360}, 60%, 45%)` }">
                  {{ agent.name[0] }}
                </div>
                <div class="agent-list-info">
                  <div class="agent-list-name">{{ agent.name }}</div>
                  <div class="agent-list-role">{{ agent.role }}</div>
                </div>
                <button class="btn-delete-sm" @click.stop="deleteAgent(agent.id)">🗑</button>
              </div>

              <button class="btn-new-agent" @click="openCreate()">＋ Thêm Agent</button>
            </div>

            <!-- Right: form -->
            <div class="agent-form">
              <h3>{{ editingAgent ? `Sửa: ${editingAgent.name}` : 'Tạo Agent Mới' }}</h3>

              <label>ID (slug, viết liền)</label>
              <input v-model="agentForm.id" :disabled="!!editingAgent" placeholder="vd: dev-backend" />

              <label>Tên hiển thị</label>
              <input v-model="agentForm.name" placeholder="vd: Backend Developer" />

              <label>Chức vụ (Role)</label>
              <input v-model="agentForm.role" placeholder="vd: Backend Developer" />

              <label>System Prompt</label>
              <textarea v-model="agentForm.system_prompt" rows="5" placeholder="Mô tả nhiệm vụ, tính cách..." />

              <label>Task Description</label>
              <textarea v-model="agentForm.task_description" rows="2" placeholder="Mô tả ngắn cho card task" />

              <div class="checkbox-container">
                <input type="checkbox" id="agent-active-cb" v-model="agentForm.active" />
                <label for="agent-active-cb">Hoạt động (Active / Làm việc)</label>
              </div>

              <div v-if="formError" class="form-error">{{ formError }}</div>

              <button class="btn-save" :disabled="saving" @click="saveAgent">
                {{ saving ? 'Đang lưu...' : editingAgent ? '💾 Cập nhật' : '✅ Tạo Agent' }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.virtual-office-layout {
  display: flex;
  height: calc(100vh - var(--header-height));
  width: 100%;
  background-color: var(--background);
  overflow: hidden;
  flex-direction: column;
}

.office-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 24px 0;
}
.office-title {
  font-weight: 700;
  font-size: 16px;
  color: var(--text-primary);
}
.btn-manage {
  background: var(--glass-bg, rgba(255,255,255,0.08));
  border: 1px solid var(--border-accent, rgba(255,255,255,0.15));
  color: var(--text-primary);
  padding: 6px 14px;
  border-radius: 8px;
  cursor: pointer;
  font-size: 13px;
  transition: background 0.2s;
}
.btn-manage:hover { background: rgba(255,255,255,0.15); }

.map-container {
  flex: 1;
  padding: 8px 24px 24px;
  overflow: auto;
  display: flex;
  flex-direction: column;
}

.office-grid {
  display: grid;
  flex: 1;
  width: 100%;
  max-width: 1200px;
  margin: 0 auto;
  min-height: 600px;
  gap: 12px;
  grid-template-columns: 1fr 1fr 1fr 1fr;
  grid-template-rows: 1fr 1fr 1fr;
  grid-template-areas:
    "meeting po_room rnd rnd"
    "meeting qa_room rnd rnd"
    "rest    game    pantry lobby";
}

.desk { width: 40px; height: 24px; background: #8b5a2b; border-radius: 2px; box-shadow: 0 4px 6px rgba(0,0,0,0.5); margin-right: 16px; }
.meeting-table { width: 120px; height: 60px; background: #334155; border-radius: 4px; box-shadow: 0 4px 6px rgba(0,0,0,0.5); margin: auto; }
.sofa { width: 60px; height: 80px; background: #9333ea; border-radius: 8px; box-shadow: 0 4px 6px rgba(0,0,0,0.5); margin: auto; }
.pool-table { width: 100px; height: 60px; background: #16a34a; border: 4px solid #78350f; border-radius: 4px; box-shadow: 0 4px 6px rgba(0,0,0,0.5); margin: auto; }
.kitchen-counter { width: 80px; height: 30px; background: #cbd5e1; border-radius: 2px; box-shadow: 0 4px 6px rgba(0,0,0,0.5); margin-left: auto; }

/* ── Popup ──────────────────────────────────────── */
.popup-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.6);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 999;
}
.popup-panel {
  background: var(--surface, #1e2130);
  border: 1px solid var(--border-accent, rgba(255,255,255,0.12));
  border-radius: 16px;
  width: 860px;
  max-width: 95vw;
  max-height: 85vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 24px 60px rgba(0,0,0,0.6);
  overflow: hidden;
}
.popup-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 24px;
  border-bottom: 1px solid var(--border-accent, rgba(255,255,255,0.1));
}
.popup-header h2 { margin: 0; font-size: 18px; color: var(--text-primary); }
.popup-close {
  background: none; border: none; color: var(--text-muted); font-size: 20px; cursor: pointer; line-height: 1;
}
.popup-body {
  display: flex;
  flex: 1;
  overflow: hidden;
}

/* Agent list (left) */
.agent-list {
  width: 240px;
  border-right: 1px solid var(--border-accent, rgba(255,255,255,0.08));
  padding: 12px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.agent-list-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border-radius: 10px;
  cursor: pointer;
  transition: background 0.15s;
}
.agent-list-item:hover, .agent-list-item.active { background: rgba(255,255,255,0.08); }
.agent-list-avatar {
  width: 32px; height: 32px; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  font-weight: 700; font-size: 14px; color: #fff; flex-shrink: 0;
}
.agent-list-info { flex: 1; min-width: 0; }
.agent-list-name { font-size: 13px; font-weight: 600; color: var(--text-primary); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.agent-list-role { font-size: 11px; color: var(--text-muted); }
.btn-delete-sm { background: none; border: none; cursor: pointer; font-size: 14px; opacity: 0.5; transition: opacity 0.15s; }
.btn-delete-sm:hover { opacity: 1; }
.btn-new-agent {
  margin-top: 8px;
  width: 100%;
  padding: 8px;
  border-radius: 8px;
  border: 1px dashed var(--border-accent, rgba(255,255,255,0.2));
  background: transparent;
  color: var(--text-muted);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.15s;
}
.btn-new-agent:hover { border-color: var(--accent, #7c3aed); color: var(--text-primary); }

/* Agent form (right) */
.agent-form {
  flex: 1;
  padding: 20px 24px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.agent-form h3 { margin: 0 0 4px; font-size: 15px; color: var(--text-primary); }
.agent-form label { font-size: 12px; color: var(--text-muted); margin-bottom: -4px; }
.agent-form input, .agent-form textarea {
  background: rgba(255,255,255,0.05);
  border: 1px solid var(--border-accent, rgba(255,255,255,0.12));
  border-radius: 8px;
  color: var(--text-primary);
  padding: 8px 12px;
  font-size: 13px;
  resize: vertical;
  width: 100%;
  box-sizing: border-box;
  font-family: inherit;
}
.agent-form input:focus, .agent-form textarea:focus {
  outline: none; border-color: var(--accent, #7c3aed);
}
.agent-form input:disabled { opacity: 0.5; cursor: not-allowed; }
.form-error { color: #f87171; font-size: 12px; }
.btn-save {
  margin-top: 4px;
  padding: 10px 20px;
  background: var(--accent, #7c3aed);
  border: none;
  border-radius: 8px;
  color: #fff;
  font-weight: 600;
  font-size: 14px;
  cursor: pointer;
  transition: opacity 0.15s;
  align-self: flex-start;
}
.btn-save:hover { opacity: 0.85; }
.btn-save:disabled { opacity: 0.5; cursor: not-allowed; }

.checkbox-container {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
}
.checkbox-container input[type="checkbox"] {
  width: auto;
  cursor: pointer;
}
.checkbox-container label {
  font-size: 13px;
  color: var(--text-primary);
  margin-bottom: 0;
  cursor: pointer;
}
</style>
