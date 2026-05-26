import { defineStore } from 'pinia'
import axios from 'axios'

export interface Workflow {
  id: number
  request: string
  status: string
  debate_rounds: number
  created_at: string
}

interface WorkflowState {
  workflows: Workflow[]
  loading: boolean
}

export const useWorkflowStore = defineStore('workflow', {
  state: (): WorkflowState => ({
    workflows: [],
    loading: false
  }),

  getters: {
    sortedWorkflows: (state): Workflow[] =>
      [...state.workflows].sort(
        (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
      )
  },

  actions: {
    async fetchWorkflows(): Promise<void> {
      this.loading = true
      try {
        const res = await axios.get<Workflow[]>('/api/workflows')
        this.workflows = res.data
      } catch (err) {
        console.error('[WorkflowStore] fetchWorkflows error:', err)
      } finally {
        this.loading = false
      }
    },

    async createWorkflow(request: string, rounds: number): Promise<number> {
      const res = await axios.post<Workflow>('/api/workflows', {
        request,
        debate_rounds: rounds
      })
      const workflow = res.data
      this.workflows.unshift(workflow)
      return workflow.id
    },

    async getWorkflow(id: number): Promise<Workflow | null> {
      try {
        const res = await axios.get<Workflow>(`/api/workflows/${id}`)
        // Update in list if present
        const idx = this.workflows.findIndex((w) => w.id === id)
        if (idx >= 0) {
          this.workflows[idx] = res.data
        } else {
          this.workflows.unshift(res.data)
        }
        return res.data
      } catch (err) {
        console.error(`[WorkflowStore] getWorkflow(${id}) error:`, err)
        return null
      }
    },

    updateWorkflowStatus(id: number, status: string): void {
      const wf = this.workflows.find((w) => w.id === id)
      if (wf) wf.status = status
    },

    async deleteWorkflow(id: number): Promise<void> {
      try {
        await axios.delete(`/api/workflows/${id}`)
        this.workflows = this.workflows.filter((w) => w.id !== id)
      } catch (err) {
        console.error(`[WorkflowStore] deleteWorkflow(${id}) error:`, err)
        throw err
      }
    },

    async syncWorkflow(id: number): Promise<{name: string, url: string}[]> {
      try {
        const res = await axios.post<{links: {name: string, url: string}[]}>(`/api/workflows/${id}/sync`)
        this.updateWorkflowStatus(id, 'synced')
        return res.data.links || []
      } catch (err) {
        console.error(`[WorkflowStore] syncWorkflow(${id}) error:`, err)
        throw err
      }
    },

    async sendFollowup(id: number, request: string): Promise<void> {
      try {
        await axios.post(`/api/workflows/${id}/followup`, { request })
        this.updateWorkflowStatus(id, 'processing')
      } catch (err) {
        console.error(`[WorkflowStore] sendFollowup(${id}) error:`, err)
        throw err
      }
    }
  }
})
