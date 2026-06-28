/*
Copyright (C) 2023-2026 QuantumNous
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { InternalToken } from '@/custom/features/internal-token'

export const Route = createFileRoute('/_authenticated/internal-token/')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()

    if (!auth.user) {
      throw redirect({
        to: '/403',
      })
    }
  },
  component: InternalToken,
})
