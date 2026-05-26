import { watch, onUnmounted, type Ref } from 'vue'
import axios from 'axios'
import { useDebateStore } from '@/stores/debate'
import { useWorkflowStore } from '@/stores/workflow'
import type { DebateMessage, Task } from '@/stores/debate'

const TERMINAL_STATUSES = new Set(['completed', 'failed'])
const RECONNECT_DELAY_MS = 3000
const MAX_RECONNECT_ATTEMPTS = 5

interface SSEEvent {
  type: string
  data: DebateMessage | Task[] | { status: string } | string
}

export function useSSE(workflowId: Ref<number | null>) {
  const debateStore = useDebateStore()
  const workflowStore = useWorkflowStore()

  let es: EventSource | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let reconnectAttempts = 0
  let stopped = false

  function cleanup() {
    if (reconnectTimer !== null) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    if (es) {
      es.close()
      es = null
    }
    debateStore.setSSEConnected(false)
  }

  async function refreshTasks(id: number) {
    try {
      const res = await axios.get<Task[]>(`/api/workflows/${id}/tasks`)
      debateStore.updateTasks(res.data ?? [])
    } catch (err) {
      console.error('[useSSE] refreshTasks error:', err)
    }
  }

  function connect(id: number) {
    cleanup()
    if (stopped) return

    const url = `/sse/workflows/${id}`
    es = new EventSource(url)

    es.onopen = () => {
      debateStore.setSSEConnected(true)
      reconnectAttempts = 0
      console.info(`[useSSE] Connected to ${url}`)
    }

    es.onmessage = (event: MessageEvent<string>) => {
      let parsed: SSEEvent
      try {
        parsed = JSON.parse(event.data) as SSEEvent
      } catch {
        // Try treating as raw string event
        console.warn('[useSSE] Non-JSON message:', event.data)
        return
      }

      switch (parsed.type) {
        case 'agent_thinking': {
          const agent = parsed.data as any
          debateStore.setThinkingAgent(agent)
          if (agent && agent.role) {
            debateStore.addSysLog(`[Debate] ${agent.role} đang suy nghĩ và gõ phím...`)
          }
          break
        }
        case 'debate_message': {
          const msg = parsed.data as DebateMessage
          debateStore.appendDebate(msg)
          debateStore.addSysLog(`[Debate] ${msg.role || msg.agent_name} vừa gửi một tin nhắn trao đổi.`)
          break
        }
        case 'task_update':
        case 'tasks_updated': {
          const tasks = parsed.data as Task[]
          debateStore.updateTasks(tasks)
          break
        }
        case 'workflow_status': {
          const payload = parsed.data as { status: string }
          const status = payload.status
          debateStore.updateActiveStatus(status)
          workflowStore.updateWorkflowStatus(id, status)

          if (TERMINAL_STATUSES.has(status)) {
            // Fetch final tasks on completion
            void refreshTasks(id)
            cleanup()
          }
          break
        }
        case 'sys_log': {
          const payload = parsed.data as { message: string }
          debateStore.addSysLog(payload.message)
          break
        }
        case 'ping':
        case 'heartbeat':
          // No-op keep-alive
          break
        default:
          console.debug('[useSSE] Unknown event type:', parsed.type)
      }
    }

    es.onerror = () => {
      debateStore.setSSEConnected(false)
      es?.close()
      es = null

      if (stopped) return

      // Check if workflow is in a terminal state
      const status = debateStore.workflowStatus
      if (TERMINAL_STATUSES.has(status)) return

      if (reconnectAttempts < MAX_RECONNECT_ATTEMPTS) {
        reconnectAttempts++
        const delay = RECONNECT_DELAY_MS * reconnectAttempts
        console.warn(`[useSSE] Connection lost. Reconnecting in ${delay}ms (attempt ${reconnectAttempts}/${MAX_RECONNECT_ATTEMPTS})`)
        reconnectTimer = setTimeout(() => {
          if (!stopped) connect(id)
        }, delay)
      } else {
        console.error('[useSSE] Max reconnection attempts reached. Giving up.')
      }
    }
  }

  // Watch workflowId and connect/disconnect accordingly
  watch(
    workflowId,
    (newId) => {
      stopped = false
      reconnectAttempts = 0
      if (newId !== null) {
        connect(newId)
      } else {
        cleanup()
      }
    },
    { immediate: true }
  )

  onUnmounted(() => {
    stopped = true
    cleanup()
  })

  return { cleanup }
}
