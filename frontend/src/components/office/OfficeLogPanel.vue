<script setup lang="ts">
import { ref } from 'vue'

defineProps<{
  roomName: string
  presentMembers: string[]
  logs: { time: string, message: string }[]
}>()

const collapsed = ref(false)
</script>

<template>
  <div class="office-log-panel" :class="{ collapsed }">
    <div class="panel-header" @click="collapsed = !collapsed">
      <div class="header-left">
        <span class="room-icon">📍</span>
        <span class="room-name">{{ roomName || 'Global' }}</span>
        <span v-if="presentMembers.length > 0" class="member-badge">{{ presentMembers.length }}</span>
      </div>
      <span class="toggle-icon">{{ collapsed ? '▲' : '▼' }}</span>
    </div>

    <div v-if="!collapsed" class="panel-body">
      <div v-if="presentMembers.length > 0" class="members-row">
        {{ presentMembers.join(', ') }}
      </div>
      <div v-else class="members-row empty">Trống</div>

      <div class="log-list">
        <div v-for="(log, idx) in logs.slice(0, 8)" :key="idx" class="log-line">
          <span class="log-time">[{{ log.time }}]</span>
          <span class="log-msg">{{ log.message }}</span>
        </div>
        <div v-if="logs.length === 0" class="empty-state">Chưa có hoạt động...</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.office-log-panel {
  position: fixed;
  bottom: 20px;
  right: 20px;
  width: 280px;
  background: rgba(10, 15, 30, 0.92);
  border: 1px solid var(--border-accent, rgba(255,255,255,0.12));
  border-radius: 12px;
  box-shadow: 0 8px 32px rgba(0,0,0,0.5);
  backdrop-filter: blur(12px);
  z-index: 100;
  overflow: hidden;
  transition: all 0.2s ease;
}

.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  cursor: pointer;
  user-select: none;
  border-bottom: 1px solid rgba(255,255,255,0.06);
  transition: background 0.15s;
}
.panel-header:hover { background: rgba(255,255,255,0.05); }

.header-left {
  display: flex;
  align-items: center;
  gap: 6px;
}
.room-icon { font-size: 13px; }
.room-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary, #e2e8f0);
  max-width: 160px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.member-badge {
  background: var(--accent, #7c3aed);
  color: #fff;
  font-size: 10px;
  font-weight: 700;
  padding: 1px 6px;
  border-radius: 99px;
  line-height: 1.6;
}
.toggle-icon {
  font-size: 10px;
  color: var(--text-muted, #94a3b8);
}

.panel-body {
  padding: 10px 14px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.members-row {
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
  line-height: 1.5;
}
.members-row.empty { font-style: italic; }

.log-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 140px;
  overflow-y: auto;
}
.log-list::-webkit-scrollbar { width: 3px; }
.log-list::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.15); border-radius: 4px; }

.log-line {
  display: flex;
  gap: 6px;
  font-size: 11px;
  line-height: 1.4;
  font-family: var(--font-mono, monospace);
}
.log-time { color: var(--text-muted, #94a3b8); flex-shrink: 0; }
.log-msg { color: var(--text-primary, #e2e8f0); }

.empty-state {
  font-size: 11px;
  color: var(--text-muted, #94a3b8);
  font-style: italic;
  text-align: center;
}

.collapsed .panel-header {
  border-bottom: none;
}
</style>
