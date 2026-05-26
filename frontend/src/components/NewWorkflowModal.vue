<script setup lang="ts">
import { ref } from 'vue'

const emit = defineEmits<{
  (e: 'submit', request: string, rounds: number): void
  (e: 'close'): void
}>()

const request = ref('')
const rounds = ref(3)
const loading = ref(false)
const error = ref('')

const EXAMPLE_PROMPTS = [
  'Build a real-time collaborative document editor with conflict resolution',
  'Create a microservices-based e-commerce platform with payment processing',
  'Design and implement a machine learning pipeline for fraud detection'
]

async function handleSubmit() {
  const trimmed = request.value.trim()
  if (!trimmed) {
    error.value = 'Please describe your project.'
    return
  }
  error.value = ''
  loading.value = true
  try {
    emit('submit', trimmed, rounds.value)
  } finally {
    loading.value = false
  }
}

function useExample(example: string) {
  request.value = example
}
</script>

<template>
  <div class="modal-overlay" @click.self="emit('close')">
    <div class="modal-box">
      <!-- Header -->
      <div style="margin-bottom: 24px;">
        <div class="modal-title">🚀 New AI Workflow</div>
        <div class="modal-subtitle">
          Describe your project and our 5-agent team will plan, architect, and build it together.
        </div>
      </div>

      <!-- Example prompts -->
      <div style="margin-bottom: 16px;">
        <div class="form-label" style="margin-bottom: 8px;">Quick Examples</div>
        <div style="display: flex; flex-direction: column; gap: 6px;">
          <button
            v-for="ex in EXAMPLE_PROMPTS"
            :key="ex"
            class="btn btn-ghost btn-sm"
            style="justify-content: flex-start; text-align: left; white-space: normal; height: auto; padding: 8px 12px;"
            @click="useExample(ex)"
          >
            <span style="margin-right: 6px; opacity: 0.6;">→</span>{{ ex }}
          </button>
        </div>
      </div>

      <!-- Project description -->
      <div class="form-group">
        <label class="form-label">Project Description</label>
        <textarea
          v-model="request"
          class="form-textarea"
          placeholder="Describe the software you want to build in detail. Include tech stack preferences, key features, constraints, or anything the agents should know…"
          :disabled="loading"
        />
        <span v-if="error" style="color: var(--rose); font-size: 12px;">{{ error }}</span>
      </div>

      <!-- Debate rounds slider -->
      <div class="form-group">
        <label class="form-label">
          Debate Rounds —
          <span style="color: var(--cyan); font-weight: 700;">{{ rounds }}</span>
        </label>
        <input
          v-model.number="rounds"
          type="range"
          class="form-range"
          min="1"
          max="5"
          step="1"
          :disabled="loading"
        />
        <div class="range-labels">
          <span>1 — Fast</span>
          <span>3 — Balanced</span>
          <span>5 — Thorough</span>
        </div>
      </div>

      <!-- Actions -->
      <div style="display: flex; gap: 12px; margin-top: 8px;">
        <button
          class="btn btn-ghost"
          style="flex: 1;"
          :disabled="loading"
          @click="emit('close')"
        >
          Cancel
        </button>
        <button
          class="btn btn-primary"
          style="flex: 2;"
          :disabled="loading || !request.trim()"
          @click="handleSubmit"
        >
          <span v-if="loading">
            <span class="typing-dots" style="display: inline-flex; gap: 3px;">
              <span /><span /><span />
            </span>
          </span>
          <span v-else>⚡ Generate Workflow</span>
        </button>
      </div>
    </div>
  </div>
</template>
