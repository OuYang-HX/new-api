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
import type { SidebarData } from '@/components/layout/types'

/**
 * Returns custom sidebar items to be merged into the main sidebar.
 * Internal Token is now placed directly in the admin group in
 * use-sidebar-data.ts (after Channels), so no custom items needed here.
 */
export function useCustomSidebarItems(): SidebarData {
  return {
    navGroups: [],
  }
}
