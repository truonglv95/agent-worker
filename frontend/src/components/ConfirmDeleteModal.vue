<script setup lang="ts">
import { ref } from 'vue'

const props = defineProps<{
  workflowId: number
}>()

const emit = defineEmits<{
  (e: 'confirm', id: number): void
  (e: 'cancel'): void
}>()

const isDeleting = ref(false)

function handleConfirm() {
  isDeleting.value = true
  emit('confirm', props.workflowId)
  // No need to set isDeleting to false here because the component unmounts on success or parent resets it
}
</script>

<template>
  <div class="modal-overlay" @click.self="emit('cancel')">
    <div class="modal-box" style="max-width: 400px;">
      <!-- Header -->
      <div style="margin-bottom: 24px;">
        <div class="modal-title" style="color: var(--rose);">⚠️ Xóa Dự Án</div>
        <div class="modal-subtitle" style="margin-top: 8px;">
          Sếp có chắc chắn muốn xóa dự án này không? Hành động này không thể hoàn tác. Toàn bộ lịch sử cuộc hội thoại và dữ liệu liên quan sẽ bị xóa vĩnh viễn.
        </div>
      </div>

      <!-- Actions -->
      <div style="display: flex; gap: 12px; justify-content: flex-end; margin-top: 32px;">
        <button 
          class="btn btn-ghost" 
          @click="emit('cancel')" 
          :disabled="isDeleting"
        >
          Hủy bỏ
        </button>
        <button 
          class="btn btn-danger" 
          style="background: var(--rose); color: white;" 
          @click="handleConfirm" 
          :disabled="isDeleting"
        >
          {{ isDeleting ? 'Đang xóa...' : 'Xóa vĩnh viễn' }}
        </button>
      </div>
    </div>
  </div>
</template>
