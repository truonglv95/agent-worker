import { defineStore } from 'pinia'
import axios from 'axios'
import type { Workflow } from './workflow'

export interface DebateMessage {
  id: number
  workflow_id: number
  agent_id: string
  agent_name: string
  role: string
  message: string
  round: number
  created_at: string
}

export interface Task {
  id: number
  workflow_id: number
  title: string
  description: string
  type: string
  assigned_to: string
  status: string
  content: string
  created_at: string
  showContent?: boolean
}

export interface ActiveWorkflow extends Workflow {
  debates: DebateMessage[]
  tasks: Task[]
}

export interface ThinkingAgent {
  agent_id: string
  agent_name: string
  role: string
}

export interface SysLog {
  id: string
  message: string
  timestamp: Date
}

interface DebateState {
  active: ActiveWorkflow | null
  sseConnected: boolean
  thinkingAgent: ThinkingAgent | null
  sysLogs: SysLog[]
}

export const useDebateStore = defineStore('debate', {
  state: (): DebateState => ({
    active: null,
    sseConnected: false,
    thinkingAgent: null,
    sysLogs: []
  }),

  getters: {
    debates: (state): DebateMessage[] => state.active?.debates ?? [],
    tasks: (state): Task[] => state.active?.tasks ?? [],
    workflowStatus: (state): string => state.active?.status ?? 'pending',

    completedTaskCount: (state): number =>
      (state.active?.tasks ?? []).filter((t) => t.status === 'completed').length,

    taskCompletionPercent: (state): number => {
      const tasks = state.active?.tasks ?? []
      if (tasks.length === 0) return 0
      const done = tasks.filter((t) => t.status === 'completed').length
      return Math.round((done / tasks.length) * 100)
    },

    roundNumbers: (state): number[] => {
      const rounds = new Set((state.active?.debates ?? []).map((d) => d.round))
      return [...rounds].sort((a, b) => a - b)
    }
  },

  actions: {
    async loadWorkflow(id: number): Promise<void> {
      try {
        const [wfRes, debatesRes, tasksRes, sysLogsRes] = await Promise.all([
          axios.get<Workflow>(`/api/workflows/${id}`),
          axios.get<DebateMessage[]>(`/api/workflows/${id}/debates`),
          axios.get<Task[]>(`/api/workflows/${id}/tasks`),
          axios.get<SysLog[]>(`/api/workflows/${id}/syslogs`)
        ])

        this.active = {
          ...wfRes.data,
          debates: debatesRes.data ?? [],
          tasks: (tasksRes.data ?? []).map((t) => ({ ...t, showContent: false }))
        }
        
        // Parse dates if necessary
        this.sysLogs = (sysLogsRes.data ?? []).map(log => ({
          ...log,
          timestamp: new Date(log.timestamp)
        }))
      } catch (err) {
        console.error(`[DebateStore] loadWorkflow(${id}) error:`, err)
        throw err
      }
    },

    async resumeWorkflow(id: number): Promise<void> {
      try {
        await axios.post(`/api/workflows/${id}/resume`)
        // Update local status so UI reflects running state immediately
        if (this.active) {
          this.active.status = 'running'
        }
      } catch (err) {
        console.error(`[DebateStore] resumeWorkflow(${id}) error:`, err)
        throw err
      }
    },

    appendDebate(msg: DebateMessage): void {
      if (!this.active) return
      // Avoid duplicates by id
      const exists = this.active.debates.some((d) => d.id === msg.id)
      if (!exists) {
        this.active.debates.push(msg)
      }
      this.thinkingAgent = null
    },

    updateTasks(tasks: Task[]): void {
      if (!this.active) return
      // Preserve showContent state when updating
      const showContentMap = new Map(this.active.tasks.map((t) => [t.id, t.showContent ?? false]))
      this.active.tasks = tasks.map((t) => ({
        ...t,
        showContent: showContentMap.get(t.id) ?? false
      }))
    },

    updateActiveStatus(status: string): void {
      if (this.active) {
        this.active.status = status
      }
    },

    setSSEConnected(v: boolean): void {
      this.sseConnected = v
    },

    setThinkingAgent(agent: ThinkingAgent | null): void {
      this.thinkingAgent = agent
    },

    addSysLog(msg: string): void {
      this.sysLogs.push({
        id: Math.random().toString(36).substr(2, 9),
        message: msg,
        timestamp: new Date()
      })
    },

    clearActive(): void {
      this.active = null
      this.sseConnected = false
      this.thinkingAgent = null
      this.sysLogs = []
    }
  }
})
