<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  id: string
  name: string
  role: string
  status?: string // e.g. "đang code", "đang chơi bi-lắc"
  active?: boolean
}>()

defineEmits<{
  (e: 'dragstart', id: string, event: DragEvent): void
}>()

// Determine colors based on role
const avatarColors = computed(() => {
  if (props.active === false) {
    // Grayed out color palette for inactive bench agents
    return { head: '#9ca3af', body: '#4b5563' }
  }
  const role = props.role.toLowerCase()
  if (role.includes('coordinator')) return { head: '#a855f7', body: '#7c3aed' }
  if (role.includes('product owner') || role.includes('po')) return { head: '#f97316', body: '#ea580c' }
  if (role.includes('tech lead') || role.includes('techlead')) return { head: '#3b82f6', body: '#1d4ed8' }
  if (role.includes('cto') || role.includes('chief technology officer')) return { head: '#ec4899', body: '#db2777' }
  if (role.includes('cio') || role.includes('chief information officer')) return { head: '#06b6d4', body: '#0891b2' }
  if (role.includes('qa') || role.includes('quality')) return { head: '#10b981', body: '#059669' }
  if (role.includes('frontend') || role.includes('fe')) return { head: '#eab308', body: '#ca8a04' }
  if (role.includes('backend') || role.includes('be')) return { head: '#6366f1', body: '#4f46e5' }
  if (role.includes('developer') || role.includes('dev')) return { head: '#f59e0b', body: '#d97706' }
  // Fallback palette based on hash of role string
  const hash = [...role].reduce((acc, c) => acc + c.charCodeAt(0), 0)
  const hues = [0, 30, 60, 120, 180, 210, 270, 300]
  const h = hues[hash % hues.length]
  return { head: `hsl(${h}, 70%, 55%)`, body: `hsl(${h}, 70%, 35%)` }
})
</script>

<template>
  <div 
    class="agent-avatar" 
    :class="{ 'is-inactive': active === false }"
    :title="`${name} (${role})\n${status || (active === false ? 'On Bench (Chờ việc)' : 'Đang làm việc')}`"
    draggable="true"
    @dragstart="$emit('dragstart', id, $event)"
  >
    <div class="pixel-head" :style="{ backgroundColor: avatarColors.head }">
      <!-- simple pixel face details -->
      <div class="eye left"></div>
      <div class="eye right"></div>
      <div class="mouth"></div>
    </div>
    <div class="pixel-body" :style="{ backgroundColor: avatarColors.body }">
      <span class="role-badge">{{ role }}</span>
    </div>
    <div v-if="status" class="status-indicator">
      {{ status }}
    </div>
  </div>
</template>

<style scoped>
.agent-avatar {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 40px;
  cursor: pointer;
  transition: transform 0.2s ease, opacity 0.2s ease;
}

.agent-avatar.is-inactive {
  opacity: 0.45;
  filter: grayscale(0.8);
}

.agent-avatar.is-inactive:hover {
  opacity: 0.8;
  filter: grayscale(0.2);
}

.agent-avatar:hover {
  transform: scale(1.1);
  z-index: 10;
}

.pixel-head {
  width: 24px;
  height: 24px;
  position: relative;
  border: 2px solid #000;
  border-radius: 4px; /* slightly rounded for a bit of friendliness, or 0 for pure 8-bit */
}

.eye {
  position: absolute;
  width: 4px;
  height: 4px;
  background-color: #000;
  top: 8px;
}
.eye.left { left: 4px; }
.eye.right { right: 4px; }

.mouth {
  position: absolute;
  width: 8px;
  height: 2px;
  background-color: #000;
  bottom: 4px;
  left: 6px;
}

.pixel-body {
  width: 32px;
  height: 20px;
  border: 2px solid #000;
  border-top: none;
  border-radius: 2px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.role-badge {
  font-size: 8px;
  font-weight: 900;
  color: #fff;
  text-shadow: 1px 1px 0 #000;
  text-transform: uppercase;
}

.status-indicator {
  position: absolute;
  top: -24px;
  background: rgba(0, 0, 0, 0.8);
  color: #fff;
  font-size: 10px;
  padding: 2px 6px;
  border-radius: 4px;
  white-space: nowrap;
  pointer-events: none;
  border: 1px solid var(--border-accent);
  box-shadow: 0 2px 4px rgba(0,0,0,0.5);
  opacity: 0;
  transition: opacity 0.2s;
}

.agent-avatar:hover .status-indicator {
  opacity: 1;
}

/* Add a tiny speech bubble tail to the status indicator */
.status-indicator::after {
  content: '';
  position: absolute;
  bottom: -4px;
  left: 50%;
  transform: translateX(-50%);
  border-width: 4px 4px 0;
  border-style: solid;
  border-color: rgba(0,0,0,0.8) transparent transparent transparent;
}
</style>
