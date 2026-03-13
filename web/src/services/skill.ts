import api from './api'
import { streamFetch } from '@/utils/sse'
import { ApiResponse } from '@/types'

export interface Skill {
  id: string
  name: string
  description: string
  category: string
  params: { name: string; type: string; description: string; required: boolean }[]
}

export const skillService = {
  list: async () => {
    const res = await api.get<ApiResponse<{ skills: Skill[] }>>('/skill/v1/list')
    return res.data.data.skills
  },

  execute: (
    skillId: string,
    params: Record<string, unknown>,
    onMessage: (type: string, content: string) => void,
    onDone: () => void
  ) => {
    streamFetch(
      '/api/skill/v1/execute',
      { skill_id: skillId, params },
      onMessage,
      onDone
    )
  },
}

