/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { api } from '@/lib/api'
import type { TokenConfig, TokenTemplate, TokenConfigFormData, DisabledChannel, ApiResponse } from './types'

export interface TokenConfigListResponse {
  success: boolean
  message?: string
  data: TokenConfig[]
  meta?: {
    reveal_allowed: boolean
  }
}

export async function getTokenConfigs(): Promise<TokenConfigListResponse> {
  const res = await api.get('/api/user/token-config/')
  return res.data
}

export async function createTokenConfig(
  data: TokenConfigFormData
): Promise<ApiResponse<TokenConfig>> {
  const res = await api.post('/api/user/token-config/', data)
  return res.data
}

export async function updateTokenConfig(
  id: number,
  data: Partial<TokenConfigFormData>
): Promise<ApiResponse<TokenConfig>> {
  const res = await api.put(`/api/user/token-config/${id}`, data)
  return res.data
}

export async function deleteTokenConfig(
  id: number
): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/token-config/${id}`)
  return res.data
}

export async function refreshTokenConfig(
  id: number
): Promise<ApiResponse<TokenConfig>> {
  const res = await api.post(`/api/user/token-config/${id}/refresh`)
  return res.data
}

// Admin-only: get all token configs across users
export async function getAllTokenConfigs(): Promise<ApiResponse<TokenConfig[]>> {
  const res = await api.get('/api/user/token-config/all')
  return res.data
}

// Token templates (readable by all users, admin-only write)
export async function getTokenTemplates(): Promise<ApiResponse<TokenTemplate[]>> {
  const res = await api.get('/api/user/token-config/templates')
  return res.data
}

export async function createTokenTemplate(
  data: Partial<TokenTemplate>
): Promise<ApiResponse<TokenTemplate>> {
  const res = await api.post('/api/user/token-config/templates', data)
  return res.data
}

export async function updateTokenTemplate(
  id: number,
  data: Partial<TokenTemplate>
): Promise<ApiResponse<TokenTemplate>> {
  const res = await api.put(`/api/user/token-config/templates/${id}`, data)
  return res.data
}

export async function deleteTokenTemplate(
  id: number
): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/token-config/templates/${id}`)
  return res.data
}

// Disabled channels — used as channel template blueprints
export async function getDisabledChannels(): Promise<ApiResponse<DisabledChannel[]>> {
  const res = await api.get('/api/user/token-config/disabled-channels')
  return res.data
}

// Rebuild channels for a template — creates channels for all TokenConfigs
export async function rebuildChannelsForTemplate(
  templateId: number
): Promise<ApiResponse<{ created: number; updated: number }>> {
  const res = await api.post(`/api/user/token-config/templates/${templateId}/rebuild-channels`)
  return res.data
}
