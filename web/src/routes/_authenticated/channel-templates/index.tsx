/*
Copyright (C) 2023-2026 QuantumNous
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { ChannelTemplates } from '@/custom/features/internal-token/channel-templates'

export const Route = createFileRoute('/_authenticated/channel-templates/')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()
    if (!auth.user || auth.user.role < ROLE.ADMIN) {
      throw redirect({ to: '/403' })
    }
  },
  component: ChannelTemplates,
})
