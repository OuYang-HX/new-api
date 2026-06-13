/*
Copyright (C) 2023-2026 QuantumNous
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Plus, Pencil, Trash2, RefreshCw } from 'lucide-react'
import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import {
  getTokenConfigs,
  getTokenTemplates,
  createTokenConfig,
  updateTokenConfig,
  deleteTokenConfig,
  refreshTokenConfig,
} from './api'
import type { TokenConfig, TokenConfigFormData, TokenTemplate } from './types'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'

const EMPTY_FORM: TokenConfigFormData = {
  name: '',
  template_id: undefined,
  username: '',
  password: '',
  enabled: 1,
}

function maskToken(token: string): string {
  if (!token) return '—'
  if (token.length <= 8) return '••••••••'
  return token.slice(0, 4) + '••••' + token.slice(-4)
}

export function InternalToken() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { auth } = useAuthStore()
  const isAdmin = auth.user?.role != null && auth.user.role >= ROLE.ADMIN

  // Data fetching
  const { data: configsData, isLoading } = useQuery({
    queryKey: ['token-configs'],
    queryFn: getTokenConfigs,
  })

  const { data: templatesData } = useQuery({
    queryKey: ['token-templates'],
    queryFn: getTokenTemplates,
  })

  const tokenConfigs = (configsData?.data ?? []) as TokenConfig[]
  const templates = (templatesData?.data ?? []) as TokenTemplate[]

  // Template lookup helper
  function getTemplateName(templateId: number): string {
    if (templateId === 0) return t('Custom')
    const tmpl = templates.find((t) => t.id === templateId)
    return tmpl ? tmpl.name : `#${templateId}`
  }

  // Dialog state
  const [formOpen, setFormOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [deletingId, setDeletingId] = useState<number | null>(null)
  const [form, setForm] = useState<TokenConfigFormData>(EMPTY_FORM)

  // Mutations
  const createMutation = useMutation({
    mutationFn: (data: TokenConfigFormData) => createTokenConfig(data),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Created successfully'))
        queryClient.invalidateQueries({ queryKey: ['token-configs'] })
        closeForm()
      } else {
        toast.error(res.message ?? t('Operation failed'))
      }
    },
    onError: () => toast.error(t('Operation failed')),
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: number; data: Partial<TokenConfigFormData> }) =>
      updateTokenConfig(id, data),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Updated successfully'))
        queryClient.invalidateQueries({ queryKey: ['token-configs'] })
        closeForm()
      } else {
        toast.error(res.message ?? t('Operation failed'))
      }
    },
    onError: () => toast.error(t('Operation failed')),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteTokenConfig(id),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Deleted successfully'))
        queryClient.invalidateQueries({ queryKey: ['token-configs'] })
        setDeleteOpen(false)
        setDeletingId(null)
      } else {
        toast.error(res.message ?? t('Operation failed'))
      }
    },
    onError: () => toast.error(t('Operation failed')),
  })

  const refreshMutation = useMutation({
    mutationFn: (id: number) => refreshTokenConfig(id),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Refreshed successfully'))
        queryClient.invalidateQueries({ queryKey: ['token-configs'] })
      } else {
        toast.error(res.message ?? t('Operation failed'))
      }
    },
    onError: () => toast.error(t('Operation failed')),
  })

  // Handlers
  function openCreate() {
    setEditingId(null)
    setForm(EMPTY_FORM)
    setFormOpen(true)
  }

  function openEdit(row: TokenConfig) {
    setEditingId(row.id)
    setForm({
      name: row.name,
      template_id: row.template_id,
      username: row.username,
      password: row.password,
      enabled: row.enabled,
    })
    setFormOpen(true)
  }

  function closeForm() {
    setFormOpen(false)
    setEditingId(null)
    setForm(EMPTY_FORM)
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (editingId !== null) {
      updateMutation.mutate({ id: editingId, data: form })
    } else {
      createMutation.mutate(form)
    }
  }

  function confirmDelete(id: number) {
    setDeletingId(id)
    setDeleteOpen(true)
  }

  function handleDelete() {
    if (deletingId !== null) {
      deleteMutation.mutate(deletingId)
    }
  }

  function updateField<K extends keyof TokenConfigFormData>(
    key: K,
    value: TokenConfigFormData[K]
  ) {
    setForm((prev) => ({ ...prev, [key]: value }))
  }

  // Whether the form should show the full custom config fields
  // Only when admin explicitly selects "Custom (no template)"
  const showCustomFields = isAdmin && form.template_id === 0

  const isSubmitting = createMutation.isPending || updateMutation.isPending

  return (
    <>
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Internal Token')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button size='sm' onClick={openCreate}>
          <Plus className='h-4 w-4' />
          {t('Create')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        {isLoading ? (
          <div className='space-y-3'>
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className='h-12 w-full' />
            ))}
          </div>
        ) : tokenConfigs.length === 0 ? (
          <div className='flex items-center justify-center py-12 text-muted-foreground'>
            {t('No data')}
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Name')}</TableHead>
                <TableHead>{t('Template')}</TableHead>
                <TableHead>{t('Username')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Current Token')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {tokenConfigs.map((row) => (
                <TableRow key={row.id}>
                  <TableCell className='font-medium'>{row.name}</TableCell>
                  <TableCell>{getTemplateName(row.template_id)}</TableCell>
                  <TableCell>{row.username || '—'}</TableCell>
                  <TableCell>
                    <Badge variant={row.enabled ? 'default' : 'secondary'}>
                      {row.enabled ? t('Enabled') : t('Disabled')}
                    </Badge>
                  </TableCell>
                  <TableCell className='font-mono text-xs'>
                    {maskToken(row.current_token)}
                  </TableCell>
                  <TableCell className='text-right'>
                    <div className='flex items-center justify-end gap-1'>
                      <Button
                        variant='ghost'
                        size='icon'
                        onClick={() => openEdit(row)}
                        title={t('Edit')}
                      >
                        <Pencil className='h-4 w-4' />
                      </Button>
                      <Button
                        variant='ghost'
                        size='icon'
                        onClick={() => refreshMutation.mutate(row.id)}
                        title={t('Refresh')}
                        disabled={refreshMutation.isPending}
                      >
                        <RefreshCw className='h-4 w-4' />
                      </Button>
                      <Button
                        variant='ghost'
                        size='icon'
                        onClick={() => confirmDelete(row.id)}
                        title={t('Delete')}
                      >
                        <Trash2 className='h-4 w-4' />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </SectionPageLayout.Content>
    </SectionPageLayout>

    {/* Create/Edit Dialog */}
    <Dialog
      open={formOpen}
      onOpenChange={(open) => !open && closeForm()}
      title={editingId !== null ? t('Edit') + ' ' + t('Internal Token') : t('Create') + ' ' + t('Internal Token')}
      contentClassName='sm:max-w-lg'
      footer={
        <div className='flex gap-2'>
          <Button type='button' variant='outline' onClick={closeForm}>
            {t('Cancel')}
          </Button>
          <Button type='button' disabled={isSubmitting} onClick={() => {
            const formEl = document.querySelector('form[data-form="token-config"]') as HTMLFormElement | null
            formEl?.requestSubmit()
          }}>
            {isSubmitting ? t('Saving...') : t('Save')}
          </Button>
        </div>
      }
    >
      <form data-form='token-config' onSubmit={handleSubmit} className='space-y-4'>
        <div className='space-y-2'>
          <Label htmlFor='template_id'>{t('Template')}</Label>
          <Select
            value={form.template_id?.toString() ?? (isAdmin ? '0' : '')}
            onValueChange={(val) => updateField('template_id', Number(val))}
          >
            <SelectTrigger id='template_id' className='w-full'>
              <SelectValue placeholder={t('Select template')} />
            </SelectTrigger>
            <SelectContent className='w-[var(--radix-select-trigger-width)]'>
              {isAdmin && (
                <SelectItem value='0'>{t('Custom (no template)')}</SelectItem>
              )}
              {templates.map((tmpl) => (
                <SelectItem key={tmpl.id} value={tmpl.id.toString()}>
                  {tmpl.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {templates.length === 0 && !isAdmin && (
            <p className='text-muted-foreground text-xs'>
              {t('No templates available. Ask your admin to create one first.')}
            </p>
          )}
        </div>

        <div className='space-y-2'>
          <Label htmlFor='name'>{t('Name')}</Label>
          <Input
            id='name'
            value={form.name}
            onChange={(e) => updateField('name', e.target.value)}
            required
          />
        </div>

        {showCustomFields && (
          <>
            <div className='space-y-2'>
              <Label htmlFor='login_url'>{t('Login URL')}</Label>
              <Input
                id='login_url'
                value={form.login_url ?? ''}
                onChange={(e) => updateField('login_url', e.target.value)}
                required
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor='login_method'>{t('Login Method')}</Label>
              <Select
                value={form.login_method ?? 'POST'}
                onValueChange={(val) => updateField('login_method', val ?? undefined)}
              >
                <SelectTrigger id='login_method'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='POST'>POST</SelectItem>
                  <SelectItem value='GET'>GET</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className='space-y-2'>
              <Label htmlFor='login_headers'>{t('Login Headers')}</Label>
              <Input
                id='login_headers'
                value={form.login_headers ?? '{}'}
                onChange={(e) => updateField('login_headers', e.target.value)}
                className='font-mono text-xs'
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor='login_body'>{t('Login Body')}</Label>
              <Input
                id='login_body'
                value={form.login_body ?? ''}
                onChange={(e) => updateField('login_body', e.target.value)}
                className='font-mono text-xs'
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor='token_json_path'>{t('Token JSON Path')}</Label>
              <Input
                id='token_json_path'
                value={form.token_json_path ?? ''}
                onChange={(e) => updateField('token_json_path', e.target.value)}
                placeholder='data.access_token'
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor='refresh_interval'>{t('Refresh Interval')} (s)</Label>
              <Input
                id='refresh_interval'
                type='number'
                min={0}
                value={form.refresh_interval ?? 3600}
                onChange={(e) =>
                  updateField('refresh_interval', Number(e.target.value))
                }
              />
            </div>
          </>
        )}

        <div className='space-y-2'>
          <Label htmlFor='username'>{t('Username')}</Label>
          <Input
            id='username'
            value={form.username ?? ''}
            onChange={(e) => updateField('username', e.target.value)}
          />
        </div>

        <div className='space-y-2'>
          <Label htmlFor='password'>{t('Password')}</Label>
          <Input
            id='password'
            type='password'
            value={form.password ?? ''}
            onChange={(e) => updateField('password', e.target.value)}
          />
        </div>

        <div className='flex items-center gap-3'>
          <Switch
            id='enabled'
            checked={form.enabled === 1}
            onCheckedChange={(checked) =>
              updateField('enabled', checked ? 1 : 0)
            }
          />
          <Label htmlFor='enabled'>{t('Enabled')}</Label>
        </div>
      </form>
    </Dialog>

    {/* Delete Confirmation */}
    <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('Confirm Delete')}</AlertDialogTitle>
          <AlertDialogDescription>
            {t('This action cannot be undone.')}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
          <AlertDialogAction onClick={handleDelete}>
            {t('Delete')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
    </>
  )
}
