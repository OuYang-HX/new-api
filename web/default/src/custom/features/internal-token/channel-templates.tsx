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
  getDisabledChannels,
  rebuildChannelsForTemplate,
} from './api'
import type { TokenTemplate, DisabledChannel } from './types'

const EMPTY_FORM = {
  name: '',
  channel_template_id: 0,
  token_template_id: 0,
}

export function ChannelTemplates() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery({
    queryKey: ['token-templates'],
    queryFn: getTokenTemplates,
  })

  const { data: disabledChannelsData, isLoading: dcLoading } = useQuery({
    queryKey: ['disabled-channels'],
    queryFn: getDisabledChannels,
  })

  const templates = (data?.data ?? []) as TokenTemplate[]
  const disabledChannels = (disabledChannelsData?.data ?? []) as DisabledChannel[]

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

  const rebuildMutation = useMutation({
    mutationFn: (templateId: number) => rebuildChannelsForTemplate(templateId),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(res.message ?? t('Channels rebuilt successfully'))
        queryClient.invalidateQueries({ queryKey: ['token-templates'] })
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
      channel_template_id: row.channel_template_id ?? 0,
      token_template_id: row.token_template_id ?? 0,
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
      <SectionPageLayout.Title>{t('Channel Templates')}</SectionPageLayout.Title>
      <SectionPageLayout.Description>
        {t('Define channel templates that auto-create per-user channels. Select a disabled channel as blueprint and a token source.')}
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
                <TableHead>{t('Channel Template')}</TableHead>
                <TableHead>{t('Token Source')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {templates.map((row) => (
                <TableRow key={row.id}>
                  <TableCell className='font-medium'>{row.name}</TableCell>
                  <TableCell>
                    {row.channel_template_id > 0 ? (
                      <span className='text-xs'>#{row.channel_template_id}</span>
                    ) : (
                      <span className='text-muted-foreground'>—</span>
                    )}
                  </TableCell>
                  <TableCell>
                    {row.token_template_id > 0 ? (
                      <span className='text-xs'>
                        {templates.find(t => t.id === row.token_template_id)?.name ?? `#${row.token_template_id}`}
                      </span>
                    ) : (
                      <span className='text-xs text-muted-foreground'>{t('Self')}</span>
                    )}
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
                      {row.channel_template_id > 0 && (
                        <Button
                          variant='ghost'
                          size='icon'
                          onClick={() => rebuildMutation.mutate(row.id)}
                          title={t('Rebuild channels')}
                          disabled={rebuildMutation.isPending}
                        >
                          <RefreshCw className='h-4 w-4' />
                        </Button>
                      )}
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
      title={editingId !== null ? t('Edit') + ' ' + t('Channel Template') : t('Create') + ' ' + t('Channel Template')}
      contentClassName='sm:max-w-lg'
      footer={
        <div className='flex gap-2'>
          <Button type='button' variant='outline' onClick={closeForm}>
            {t('Cancel')}
          </Button>
          <Button type='button' disabled={isSubmitting} onClick={() => {
            const formEl = document.querySelector('form[data-form="channel-template"]') as HTMLFormElement | null
            formEl?.requestSubmit()
          }}>
            {isSubmitting ? t('Saving...') : t('Save')}
          </Button>
        </div>
      }
    >
      <form data-form='channel-template' onSubmit={handleSubmit} className='space-y-4'>
        <div className='space-y-2'>
          <Label htmlFor='ct-name'>{t('Name')}</Label>
          <Input
            id='ct-name'
            value={form.name}
            onChange={(e) => updateField('name', e.target.value)}
            required
            placeholder='e.g. xunfei-maas'
          />
        </div>

        <div className='border-t pt-4 mt-2'>
          <h4 className='font-medium mb-3'>{t('Channel Blueprint')}</h4>
          <p className='text-muted-foreground text-xs mb-3'>
            {t('Select a disabled channel as a template. Per-user channels will be cloned from it.')}
          </p>
          <div className='space-y-2'>
            <Label htmlFor='ct-channel_template_id'>{t('Template Channel')}</Label>
            <Select
              value={form.channel_template_id > 0 ? String(form.channel_template_id) : '__none__'}
              onValueChange={(val) => updateField('channel_template_id', val === '__none__' ? 0 : Number(val))}
            >
              <SelectTrigger id='ct-channel_template_id' className='w-full'>
                <SelectValue placeholder={t('No channel template')} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='__none__'>{t('None (no auto-channel)')}</SelectItem>
                {dcLoading ? (
                  <SelectItem value='__loading__' disabled>{t('Loading...')}</SelectItem>
                ) : disabledChannels.length === 0 ? (
                  <SelectItem value='__empty__' disabled>{t('No disabled channels')}</SelectItem>
                ) : (
                  disabledChannels.map((ch) => (
                    <SelectItem key={ch.id} value={String(ch.id)}>
                      {ch.name} (ID: {ch.id}, Type: {ch.type})
                    </SelectItem>
                  ))
                )}
              </SelectContent>
            </Select>
          </div>
        </div>

        <div className='border-t pt-4 mt-2'>
          <h4 className='font-medium mb-3'>{t('Token Source')}</h4>
          <p className='text-muted-foreground text-xs mb-3'>
            {t('Select which template provides the login config for token refresh. Default uses own login config if available.')}
          </p>
          <div className='space-y-2'>
            <Label htmlFor='ct-token_template_id'>{t('Token Template')}</Label>
            <Select
              value={form.token_template_id > 0 ? String(form.token_template_id) : '__self__'}
              onValueChange={(val) => updateField('token_template_id', val === '__self__' ? 0 : Number(val))}
            >
              <SelectTrigger id='ct-token_template_id' className='w-full'>
                <SelectValue placeholder={t('Self (use own login config)')} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='__self__'>{t('Self (use own login config)')}</SelectItem>
                {templates
                  .filter(t => t.id !== editingId && t.login_url)
                  .map((tmpl) => (
                    <SelectItem key={tmpl.id} value={String(tmpl.id)}>
                      {tmpl.name}
                    </SelectItem>
                  ))}
              </SelectContent>
            </Select>
          </div>
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
