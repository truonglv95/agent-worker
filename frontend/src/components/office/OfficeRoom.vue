<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  name: string
  color: 'purple' | 'green' | 'blue' | 'yellow'
  gridArea?: string // Allows manual grid placement if needed
}>()

defineEmits<{
  (e: 'drop-agent', event: DragEvent): void
}>()

const borderColor = computed(() => {
  switch (props.color) {
    case 'purple': return '#a855f7' // var(--purple) approx
    case 'green': return '#10b981' // var(--emerald) approx
    case 'blue': return '#3b82f6' // var(--blue) approx
    case 'yellow': return '#eab308' // var(--amber) approx
    default: return '#3b82f6'
  }
})

const shadowGlow = computed(() => `0 0 10px ${borderColor.value}33, inset 0 0 10px ${borderColor.value}33`)
</script>

<template>
  <div 
    class="office-room" 
    :style="{ 
      borderColor: borderColor,
      boxShadow: shadowGlow,
      gridArea: gridArea
    }"
    @dragover.prevent
    @drop.prevent="$emit('drop-agent', $event)"
  >
    <div class="room-label" :style="{ color: borderColor }">
      {{ name }}
    </div>
    <div class="room-content">
      <slot></slot>
    </div>
  </div>
</template>

<style scoped>
.office-room {
  position: relative;
  border: 2px solid;
  border-radius: 8px;
  background: rgba(10, 15, 30, 0.4); /* Dark translucent background */
  backdrop-filter: blur(4px);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  transition: all 0.3s ease;
}

.office-room:hover {
  background: rgba(10, 15, 30, 0.6);
}

.room-label {
  position: absolute;
  top: 8px;
  left: 12px;
  font-family: var(--font-mono);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.5px;
  text-transform: uppercase;
  z-index: 2;
  text-shadow: 0 0 4px rgba(0,0,0,0.8);
  /* The image has vertical text for some rooms, but horizontal is easier to read unless explicitly needed. 
     We'll stick to horizontal but rotate if grid is tall? Let's stick to horizontal top-left */
}

/* In the provided image, some labels are rotated. We can support an optional vertical label if needed, 
   but top-left horizontal is standard. Let's make it vertical left if it's a tall room?
   Actually, the image has labels like "CEO's Office" horizontal, but "Meeting Room" vertical. 
   We will keep it simple and readable with horizontal top-left. */

.room-content {
  flex: 1;
  position: relative;
  padding: 24px 12px 12px; /* space for label */
  display: flex;
  flex-wrap: wrap;
  align-content: flex-start;
  gap: 16px;
}

/* Add grid pattern background to the room to make it look like floor tiles */
.room-content::before {
  content: '';
  position: absolute;
  top: 0; left: 0; right: 0; bottom: 0;
  background-image: 
    linear-gradient(rgba(255, 255, 255, 0.03) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255, 255, 255, 0.03) 1px, transparent 1px);
  background-size: 20px 20px;
  z-index: 0;
  pointer-events: none;
}
</style>
