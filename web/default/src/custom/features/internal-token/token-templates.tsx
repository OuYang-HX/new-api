/*
Copyright (C) 2023-2026 QuantumNous
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Plus, Pencil, Trash2 } from 'lucide-react'
import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
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
import { Skeleton } from '@/components/ui/skeleton'
import {
  getTokenTemplates,
  createTokenTemplate,
  updateTokenTemplate,
  deleteTokenTemplate,
} from './api'
import type { TokenTemplate } from './types'

const EMPTY_FORM = {
  name: '',
  login_url: '',
  login_method: 'POST',
  login_headers: '{}',
  login_body: '',
  token_json_path: '',
  refresh_interval: 3600,
}

export function TokenTemplates() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery({
    queryKey: ['token-templates'],
    queryFn: getTokenTemplates,
  })

  const templates = ((data?.data ?? []) as TokenTemplate[]).filter(t => t.login_url)

  const [formOpen, setFormOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [deletingId, setDeletingId] = useState<number | null>(null)
  const [form, setForm] = useState(EMPTY_FORM)

  const createMutation = useMutation({
    mutationFn: (data: typeof EMPTY_FORM) => createTokenTemplate(data),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Created successfully'))
        queryClient.invalidateQueries({ queryKey: ['token-templates'] })
        closeForm()
      } else {
        toast.error(res.message ?? t('Operation failed'))
      }
    },
    onError: () => toast.error(t('Operation failed')),
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: number; data: typeof EMPTY_FORM }) =>
      updateTokenTemplate(id, data),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Updated successfully'))
        queryClient.invalidateQueries({ queryKey: ['token-templates'] })
        closeForm()
      } else {
        toast.error(res.message ?? t('Operation failed'))
      }
    },
    onError: () => toast.error(t('Operation failed')),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteTokenTemplate(id),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Deleted successfully'))
        queryClient.invalidateQueries({ queryKey: ['token-templates'] })
        setDeleteOpen(false)
        setDeletingId(null)
      } else {
        toast.error(res.message ?? t('Operation failed'))
      }
    },
    onError: () => toast.error(t('Operation failed')),
  })

  function openCreate() {
    setEditingId(null)
    setForm(EMPTY_FORM)
    setFormOpen(true)
  }

  function openEdit(row: TokenTemplate) {
    setEditingId(row.id)
    setForm({
      name: row.name,
      login_url: row.login_url,
      login_method: row.login_method,
      login_headers: row.login_headers,
      login_body: row.login_body,
      token_json_path: row.token_json_path,
      refresh_interval: row.refresh_interval,
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

  function updateField<K extends keyof typeof EMPTY_FORM>(
    key: K,
    value: (typeof EMPTY_FORM)[K]
  ) {
    setForm((prev) => ({ ...prev, [key]: value }))
  }

  const isSubmitting = createMutation.isPending || updateMutation.isPending

  return (
    <>
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Token Templates')}</SectionPageLayout.Title>
      <SectionPageLayout.Description>
        {t('Define login templates that users can select when creating internal tokens. Users only need to provide their credentials.')}
      </SectionPageLayout.Description>
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
        ) : templates.length === 0 ? (
          <div className='flex items-center justify-center py-12 text-muted-foreground'>
            {t('No data')}
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Name')}</TableHead>
                <TableHead>{t('Login URL')}</TableHead>
                <TableHead>{t('Method')}</TableHead>
                <TableHead>{t('Refresh Interval')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {templates.map((row) => (
                <TableRow key={row.id}>
                  <TableCell className='font-medium'>{row.name}</TableCell>
                  <TableCell className='max-w-[300px] truncate'>{row.login_url}</TableCell>
                  <TableCell>{row.login_method}</TableCell>
                  <TableCell>
                    {row.refresh_interval >= 3600
                      ? `${(row.refresh_interval / 3600).toFixed(1)}h`
                      : `${row.refresh_interval}s`}
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
      title={editingId !== null ? t('Edit') + ' ' + t('Token Template') : t('Create') + ' ' + t('Token Template')}
      contentClassName='sm:max-w-lg'
      footer={
        <div className='flex gap-2'>
          <Button type='button' variant='outline' onClick={closeForm}>
            {t('Cancel')}
          </Button>
          <Button type='button' disabled={isSubmitting} onClick={() => {
            const formEl = document.querySelector('form[data-form="token-template"]') as HTMLFormElement | null
            formEl?.requestSubmit()
          }}>
            {isSubmitting ? t('Saving...') : t('Save')}
          </Button>
        </div>
      }
    >
      <form data-form='token-template' onSubmit={handleSubmit} className='space-y-4'>
        <div className='space-y-2'>
          <Label htmlFor='tmpl-name'>{t('Name')}</Label>
          <Input
            id='tmpl-name'
            value={form.name}
            onChange={(e) => updateField('name', e.target.value)}
            required
            placeholder='e.g. xunfei-maas'
          />
        </div>
        <div className='space-y-2'>
          <Label htmlFor='tmpl-login_url'>{t('Login URL')}</Label>
          <Input
            id='tmpl-login_url'
            value={form.login_url}
            onChange={(e) => updateField('login_url', e.target.value)}
            required
            placeholder='https://example.com/auth/token'
          />
        </div>
        <div className='space-y-2'>
          <Label htmlFor='tmpl-login_method'>{t('Login Method')}</Label>
          <Select
            value={form.login_method}
            onValueChange={(val) => updateField('login_method', val ?? 'POST')}
          >
            <SelectTrigger id='tmpl-login_method'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='POST'>POST</SelectItem>
              <SelectItem value='GET'>GET</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className='space-y-2'>
          <Label htmlFor='tmpl-login_headers'>{t('Login Headers')}</Label>
          <Textarea
            id='tmpl-login_headers'
            value={form.login_headers}
            onChange={(e) => updateField('login_headers', e.target.value)}
            rows={2}
            className='font-mono text-xs'
            placeholder='{"Content-Type": "application/json"}'
          />
        </div>
        <div className='space-y-2'>
          <Label htmlFor='tmpl-login_body'>{t('Login Body')}</Label>
          <Textarea
            id='tmpl-login_body'
            value={form.login_body}
            onChange={(e) => updateField('login_body', e.target.value)}
            rows={2}
            className='font-mono text-xs'
            placeholder='{"username": "{username}", "password": "{password}"}'
          />
        </div>
        <div className='space-y-2'>
          <Label htmlFor='tmpl-token_json_path'>{t('Token JSON Path')}</Label>
          <Input
            id='tmpl-token_json_path'
            value={form.token_json_path}
            onChange={(e) => updateField('token_json_path', e.target.value)}
            placeholder='data.access_token'
          />
        </div>
        <div className='space-y-2'>
          <Label htmlFor='tmpl-refresh_interval'>{t('Refresh Interval')} (s)</Label>
          <Input
            id='tmpl-refresh_interval'
            type='number'
            min={0}
            value={form.refresh_interval}
            onChange={(e) => updateField('refresh_interval', Number(e.target.value))}
          />
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
