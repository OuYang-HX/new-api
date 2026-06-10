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
import { useQuery } from '@tanstack/react-query'
import { KeyRound } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { Skeleton } from '@/components/ui/skeleton'
import { getTokenConfigs } from './api'
import type { TokenConfig } from './types'

interface TokenPickerProps {
  onSelect: (placeholder: string) => void
}

export function TokenPicker({ onSelect }: TokenPickerProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ['token-configs'],
    queryFn: getTokenConfigs,
    enabled: open,
  })

  const tokens: TokenConfig[] = data?.data ?? []

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button type='button' variant='outline' size='sm'>
          <KeyRound className='mr-1 h-3.5 w-3.5' />
          {t('Insert Token')}
        </Button>
      </PopoverTrigger>
      <PopoverContent className='w-64 p-0' align='end'>
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
            <p className='text-muted-foreground p-3 text-center text-xs'>
              {t('No tokens configured')}
            </p>
          ) : (
            tokens.map((token) => (
              <button
                key={token.id}
                type='button'
                className='hover:bg-muted flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm transition-colors'
                onClick={() => {
                  onSelect(`\${token:${token.name}}`)
                  setOpen(false)
                }}
              >
                <KeyRound className='text-muted-foreground h-3.5 w-3.5 shrink-0' />
                <span className='truncate font-mono text-xs'>
                  {token.name}
                </span>
                {token.enabled === 1 ? (
                  <span className='ml-auto text-[10px] text-green-600'>●</span>
                ) : (
                  <span className='text-muted-foreground ml-auto text-[10px]'>○</span>
                )}
              </button>
            ))
          )}
        </div>
      </PopoverContent>
    </Popover>
  )
}
