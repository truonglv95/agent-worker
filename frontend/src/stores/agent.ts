import { defineStore } from 'pinia'
import axios from 'axios'

export interface Agent {
  id: string
  name: string
  role: string
  system_prompt: string
  task_description: string
}

interface AgentState {
  agents: Agent[]
  loading: boolean
}

export const useAgentStore = defineStore('agent', {
  state: (): AgentState => ({
    agents: [],
    loading: false
  }),

  actions: {
    async fetchAgents(): Promise<void> {
      this.loading = true
      try {
        const res = await axios.get<Agent[]>('/api/agents')
        this.agents = res.data
      } catch (err) {
        console.error('[AgentStore] fetchAgents error:', err)
      } finally {
        this.loading = false
      }
    },

    async createAgent(payload: Omit<Agent, 'id'> & { id: string }): Promise<void> {
      const res = await axios.post<Agent>('/api/agents', payload)
      this.agents.push(res.data)
    },

    async updateAgent(id: string, payload: Partial<Agent>): Promise<void> {
      const res = await axios.put<Agent>(`/api/agents/${id}`, payload)
      const idx = this.agents.findIndex((a) => a.id === id)
      if (idx >= 0) this.agents[idx] = res.data
    },

    async deleteAgent(id: string): Promise<void> {
      await axios.delete(`/api/agents/${id}`)
      this.agents = this.agents.filter((a) => a.id !== id)
    }
  }
})
