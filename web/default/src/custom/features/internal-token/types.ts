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
import { z } from 'zod'

export const tokenConfigSchema = z.object({
  id: z.number(),
  user_id: z.number(),
  template_id: z.number(),
  username: z.string(),
  login_url: z.string().optional(),
  login_method: z.string().default('POST'),
  login_headers: z.string().optional(),
  login_body: z.string().optional(),
  password: z.string().optional(),
  token_json_path: z.string().optional(),
  refresh_interval: z.number().default(3600),
  current_token: z.string().default(''),
  token_expires_at: z.number().default(0),
  enabled: z.number().default(1),
  channel_id: z.number().default(0),
  created_time: z.number(),
  updated_time: z.number(),
})

export type TokenConfig = z.infer<typeof tokenConfigSchema>

export const tokenTemplateSchema = z.object({
  id: z.number(),
  name: z.string(),
  login_url: z.string(),
  login_method: z.string().default('POST'),
  login_headers: z.string().default('{}'),
  login_body: z.string().default(''),
  token_json_path: z.string().default(''),
  refresh_interval: z.number().default(3600),
  // Channel template: reference to a disabled Channel that serves as blueprint
  channel_template_id: z.number().default(0),
  // Which template's token to use (0 = self)
  token_template_id: z.number().default(0),
  created_time: z.number(),
  updated_time: z.number(),
})

export type TokenTemplate = z.infer<typeof tokenTemplateSchema>

export interface DisabledChannel {
  id: number
  name: string
  type: number
}

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface TokenConfigFormData {
  template_id?: number
  username: string
  password?: string
  login_url?: string
  login_method?: string
  login_headers?: string
  login_body?: string
  token_json_path?: string
  refresh_interval?: number
  enabled?: number
}
