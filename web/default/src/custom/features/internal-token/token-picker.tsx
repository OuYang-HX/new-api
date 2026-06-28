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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { KeyRound, Plus, Pencil, ChevronLeft } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { Skeleton } from '@/components/ui/skeleton'
import {
  getTokenConfigs,
  getAllTokenConfigs,
  createTokenConfig,
  updateTokenConfig,
} from './api'
import type { TokenConfig, TokenConfigFormData } from './types'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'

const EMPTY_FORM: TokenConfigFormData = {
  name: '',
  login_url: '',
  login_method: 'POST',
  login_headers: '{}',
  login_body: '',
  username: '',
  password: '',
  token_json_path: '',
  refresh_interval: 3600,
  enabled: 1,
}

interface TokenPickerProps {
  onSelect: (placeholder: string) => void
}

type View = 'list' | 'create' | 'edit'

export function TokenPicker({ onSelect }: TokenPickerProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { auth } = useAuthStore()
  const isAdmin = auth.user?.role != null && auth.user.role >= ROLE.ADMIN
  const [open, setOpen] = useState(false)
  const [view, setView] = useState<View>('list')
  const [editingId, setEditingId] = useState<number | null>(null)
  const [form, setForm] = useState<TokenConfigFormData>(EMPTY_FORM)

  const { data, isLoading } = useQuery({
    queryKey: ['token-configs', isAdmin ? 'all' : 'own'],
    queryFn: isAdmin ? getAllTokenConfigs : getTokenConfigs,
    enabled: open,
  })

  const tokens: TokenConfig[] = data?.data ?? []

  const createMutation = useMutation({
    mutationFn: (d: TokenConfigFormData) => createTokenConfig(d),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Created successfully'))
        queryClient.invalidateQueries({ queryKey: ['token-configs'] })
        resetForm()
      } else {
        toast.error(res.message ?? t('Operation failed'))
      }
    },
    onError: () => toast.error(t('Operation failed')),
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data: d }: { id: number; data: Partial<TokenConfigFormData> }) =>
      updateTokenConfig(id, d),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Updated successfully'))
        queryClient.invalidateQueries({ queryKey: ['token-configs'] })
        resetForm()
      } else {
        toast.error(res.message ?? t('Operation failed'))
      }
    },
    onError: () => toast.error(t('Operation failed')),
  })

  const isSubmitting = createMutation.isPending || updateMutation.isPending

  function resetForm() {
    setView('list')
    setEditingId(null)
    setForm(EMPTY_FORM)
  }

  function openCreate() {
    setEditingId(null)
    setForm(EMPTY_FORM)
    setView('create')
  }

  function openEdit(token: TokenConfig) {
    setEditingId(token.id)
    setForm({
      name: token.name,
      login_url: token.login_url,
      login_method: token.login_method,
      login_headers: token.login_headers,
      login_body: token.login_body,
      username: token.username,
      password: token.password,
      token_json_path: token.token_json_path,
      refresh_interval: token.refresh_interval,
      enabled: token.enabled,
    })
    setView('edit')
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (editingId !== null) {
      updateMutation.mutate({ id: editingId, data: form })
    } else {
      createMutation.mutate(form)
    }
  }

  function updateField<K extends keyof TokenConfigFormData>(
    key: K,
    value: TokenConfigFormData[K]
  ) {
    setForm((prev) => ({ ...prev, [key]: value }))
  }

  function handleOpenChange(nextOpen: boolean) {
    setOpen(nextOpen)
    if (!nextOpen) resetForm()
  }

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <PopoverTrigger asChild>
        <Button type='button' variant='outline' size='sm'>
          <KeyRound className='mr-1 h-3.5 w-3.5' />
          {t('Insert Token')}
        </Button>
      </PopoverTrigger>
      <PopoverContent className='w-80 p-0' align='end'>
        {view === 'list' && (
          <>
            <div className='border-b px-3 py-2'>
              <p className='text-sm font-medium'>{t('Internal Tokens')}</p>
              <p className='text-muted-foreground text-xs'>
                {t('Select a token to insert as ${token:name}')}
              </p>
            </div>
            <div className='max-h-60 overflow-y-auto p-1'>
              {isLoading ? (
                <div className='space-y-2 p-2'>
                  <Skeleton className='h-6 w-full' />
                  <Skeleton className='h-6 w-full' />
                </div>
              ) : tokens.length === 0 ? (
                <div className='flex flex-col items-center gap-2 py-6'>
                  <p className='text-muted-foreground text-xs'>
                    {t('No tokens configured')}
                  </p>
                  <Button type='button' variant='outline' size='sm' onClick={openCreate}>
                    <Plus className='mr-1 h-3.5 w-3.5' />
                    {t('Create Token')}
                  </Button>
                </div>
              ) : (
                tokens.map((token) => (
                  <div
                    key={token.id}
                    className='hover:bg-muted flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm transition-colors'
                  >
                    <button
                      type='button'
                      className='flex flex-1 items-center gap-2 text-left'
                      onClick={() => {
                        onSelect(`\${token:${token.name}}`)
                        setOpen(false)
                      }}
                    >
                      <KeyRound className='text-muted-foreground h-3.5 w-3.5 shrink-0' />
                      <span className='truncate font-mono text-xs'>
                        {token.name}
                      </span>
                      {isAdmin && token.user_id !== auth.user?.id && (
                        <span className='text-muted-foreground ml-1 text-[10px]'>
                          (uid:{token.user_id})
                        </span>
                      )}
                      {token.enabled === 1 ? (
                        <span className='ml-auto text-[10px] text-green-600'>●</span>
                      ) : (
                        <span className='text-muted-foreground ml-auto text-[10px]'>○</span>
                      )}
                    </button>
                    {token.user_id === auth.user?.id && (
                      <button
                        type='button'
                        className='text-muted-foreground hover:text-foreground shrink-0 rounded p-0.5 transition-colors'
                        onClick={(e) => { e.stopPropagation(); openEdit(token) }}
                        title={t('Edit')}
                      >
                        <Pencil className='h-3 w-3' />
                      </button>
                    )}
                  </div>
                ))
              )}
            </div>
            {tokens.length > 0 && (
              <div className='border-t p-2'>
                <Button type='button' variant='ghost' size='sm' className='w-full' onClick={openCreate}>
                  <Plus className='mr-1 h-3.5 w-3.5' />
                  {t('Create Token')}
                </Button>
              </div>
            )}
          </>
        )}

        {(view === 'create' || view === 'edit') && (
          <form onSubmit={handleSubmit} className='max-h-[70vh] overflow-y-auto p-3'>
            <div className='mb-3 flex items-center gap-2'>
              <button
                type='button'
                className='text-muted-foreground hover:text-foreground rounded p-0.5 transition-colors'
                onClick={resetForm}
              >
                <ChevronLeft className='h-4 w-4' />
              </button>
              <p className='text-sm font-medium'>
                {view === 'create' ? t('Create Token') : t('Edit Token')}
              </p>
            </div>

            <div className='space-y-3'>
              <div className='space-y-1'>
                <Label className='text-xs'>{t('Name')}</Label>
                <Input
                  value={form.name}
                  onChange={(e) => updateField('name', e.target.value)}
                  required
                  className='h-8 text-xs'
                />
              </div>

              <div className='space-y-1'>
                <Label className='text-xs'>{t('Login URL')}</Label>
                <Input
                  value={form.login_url}
                  onChange={(e) => updateField('login_url', e.target.value)}
                  required
                  className='h-8 text-xs'
                  placeholder='https://example.com/auth/token'
                />
              </div>

              <div className='space-y-1'>
                <Label className='text-xs'>{t('Login Method')}</Label>
                <Select
                  value={form.login_method}
                  onValueChange={(val) => updateField('login_method', val ?? undefined)}
                >
                  <SelectTrigger className='h-8 text-xs'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='POST'>POST</SelectItem>
                    <SelectItem value='GET'>GET</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className='space-y-1'>
                <Label className='text-xs'>{t('Login Headers')}</Label>
                <Textarea
                  value={form.login_headers}
                  onChange={(e) => updateField('login_headers', e.target.value)}
                  rows={2}
                  className='font-mono text-xs'
                  placeholder='{"Content-Type": "application/json"}'
                />
              </div>

              <div className='space-y-1'>
                <Label className='text-xs'>{t('Login Body')}</Label>
                <Textarea
                  value={form.login_body}
                  onChange={(e) => updateField('login_body', e.target.value)}
                  rows={2}
                  className='font-mono text-xs'
                />
              </div>

              <div className='grid grid-cols-2 gap-2'>
                <div className='space-y-1'>
                  <Label className='text-xs'>{t('Username')}</Label>
                  <Input
                    value={form.username}
                    onChange={(e) => updateField('username', e.target.value)}
                    className='h-8 text-xs'
                  />
                </div>
                <div className='space-y-1'>
                  <Label className='text-xs'>{t('Password')}</Label>
                  <Input
                    type='password'
                    value={form.password}
                    onChange={(e) => updateField('password', e.target.value)}
                    className='h-8 text-xs'
                  />
                </div>
              </div>

              <div className='space-y-1'>
                <Label className='text-xs'>{t('Token JSON Path')}</Label>
                <Input
                  value={form.token_json_path}
                  onChange={(e) => updateField('token_json_path', e.target.value)}
                  className='h-8 text-xs'
                  placeholder='data.access_token'
                />
              </div>

              <div className='space-y-1'>
                <Label className='text-xs'>{t('Refresh Interval')} (s)</Label>
                <Input
                  type='number'
                  min={0}
                  value={form.refresh_interval}
                  onChange={(e) => updateField('refresh_interval', Number(e.target.value))}
                  className='h-8 text-xs'
                />
              </div>

              <div className='flex items-center gap-2'>
                <Switch
                  checked={form.enabled === 1}
                  onCheckedChange={(checked) => updateField('enabled', checked ? 1 : 0)}
                />
                <Label className='text-xs'>{t('Enabled')}</Label>
              </div>
            </div>

            <div className='mt-4 flex justify-end gap-2'>
              <Button type='button' variant='outline' size='sm' onClick={resetForm}>
                {t('Cancel')}
              </Button>
              <Button type='submit' size='sm' disabled={isSubmitting}>
                {isSubmitting ? t('Saving...') : t('Save')}
              </Button>
            </div>
          </form>
        )}
      </PopoverContent>
    </Popover>
  )
}
