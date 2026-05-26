<script setup lang="ts">
import { ref, watch, nextTick, computed } from 'vue'
import { useDebateStore } from '@/stores/debate'

const debateStore = useDebateStore()
const isMinimized = ref(false)
const containerRef = ref<HTMLElement | null>(null)

const sysLogs = computed(() => debateStore.sysLogs)

// Auto-scroll to bottom when logs change
watch(
  () => sysLogs.value.length,
  async () => {
    if (isMinimized.value) return
    await nextTick()
    if (containerRef.value) {
      containerRef.value.scrollTop = containerRef.value.scrollHeight
    }
  }
)

function toggleMinimize() {
  isMinimized.value = !isMinimized.value
  if (!isMinimized.value) {
    nextTick(() => {
      if (containerRef.value) {
        containerRef.value.scrollTop = containerRef.value.scrollHeight
      }
    })
  }
}

// Simple coloring logic based on keywords
function getLogColor(msg: string): string {
  const lower = msg.toLowerCase()
  if (lower.includes('error') || lower.includes('fail') || lower.includes('rejected')) {
    return 'var(--rose)' // Red for errors
  }
  if (lower.includes('success') || lower.includes('completed') || lower.includes('passed')) {
    return 'var(--emerald)' // Green for success
  }
  if (lower.includes('clone') || lower.includes('git') || lower.includes('push')) {
    return 'var(--cyan)' // Cyan for git ops
  }
  if (lower.includes('qa') || lower.includes('test')) {
    return 'var(--amber)' // Yellow for QA
  }
  if (lower.includes('devrunner') || lower.includes('local server')) {
    return 'var(--violet)' // Violet for server
  }
  return 'var(--text-secondary)' // Default
}

function formatTime(date: Date): string {
  return date.toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })
}
</script>

<template>
  <div class="system-logs-dock" :class="{ 'is-minimized': isMinimized }">
    <!-- Header -->
    <div class="dock-header" @click="toggleMinimize">
      <div class="dock-header__left">
        <span class="dock-icon">💻</span>
        <span class="dock-title">System Activity</span>
        <span v-if="isMinimized && sysLogs.length > 0" class="dock-badge">{{ sysLogs.length }}</span>
      </div>
      <button class="btn btn-ghost btn-icon">
        {{ isMinimized ? '▲' : '▼' }}
      </button>
    </div>

    <!-- Body -->
    <div v-show="!isMinimized" ref="containerRef" class="dock-body">
      <div v-if="sysLogs.length === 0" class="empty-log">
        Waiting for system events...
      </div>
      <div 
        v-for="log in sysLogs" 
        :key="log.id" 
        class="log-line"
      >
        <span class="log-time">[{{ formatTime(log.timestamp) }}]</span>
        <span class="log-msg" :style="{ color: getLogColor(log.message) }">> {{ log.message }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.system-logs-dock {
  position: fixed;
  bottom: 24px;
  right: 24px;
  width: 450px;
  max-width: calc(100vw - 48px);
  background: rgba(10, 15, 30, 0.95);
  backdrop-filter: blur(20px);
  border: 1px solid var(--border-accent);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-lg), var(--cyan-glow);
  display: flex;
  flex-direction: column;
  transition: all var(--transition-base);
  z-index: 1000;
  overflow: hidden;
}

.system-logs-dock.is-minimized {
  width: 250px;
  box-shadow: var(--shadow-md);
  border-color: var(--border);
}

.system-logs-dock.is-minimized:hover {
  border-color: var(--border-accent);
}

.dock-header {
  padding: 10px 14px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  cursor: pointer;
  background: rgba(0, 0, 0, 0.2);
  border-bottom: 1px solid var(--border);
  user-select: none;
}

.dock-header__left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.dock-icon {
  font-size: 14px;
}

.dock-title {
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.5px;
  text-transform: uppercase;
  color: var(--text-primary);
}

.dock-badge {
  background: var(--cyan);
  color: #fff;
  font-size: 10px;
  font-weight: 700;
  padding: 2px 6px;
  border-radius: 99px;
  margin-left: 4px;
}

.dock-body {
  height: 300px;
  overflow-y: auto;
  padding: 12px;
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: 1.5;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.dock-body::-webkit-scrollbar {
  width: 4px;
}

.dock-body::-webkit-scrollbar-thumb {
  background: var(--border-accent);
  border-radius: 99px;
}

.empty-log {
  color: var(--text-muted);
  font-style: italic;
  text-align: center;
  margin-top: 40px;
}

.log-line {
  display: flex;
  gap: 8px;
  word-break: break-word;
}

.log-time {
  color: var(--text-muted);
  flex-shrink: 0;
}

.log-msg {
  flex: 1;
}

.btn-icon {
  padding: 4px;
  font-size: 10px;
  color: var(--text-secondary);
}
</style>
