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
import { useTranslation } from 'react-i18next'
import { getLobeIcon } from '@/lib/lobe-icon'
import { HERO_PROVIDERS } from '../constants'

interface ProviderGridProps {
  className?: string
}

/**
 * Provider showcase: a responsive grid of large-model providers plus a trailing
 * "30+" tile. Icons come from @lobehub/icons via `getLobeIcon`, which renders a
 * stable letter badge when an icon is unavailable (no remote images).
 */
export function ProviderGrid(props: ProviderGridProps) {
  const { t } = useTranslation()

  return (
    <section className={props.className}>
      <div className='mx-auto max-w-6xl px-6'>
        <h2 className='text-center text-xl font-bold tracking-tight md:text-2xl'>
          {t('Supports a vast range of large-model providers')}
        </h2>

        <div className='mt-10 grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-8'>
          {HERO_PROVIDERS.map((provider) => (
            <div
              key={provider.name}
              className='border-border/50 bg-card/40 hover:border-border hover:bg-card/70 flex flex-col items-center justify-center gap-2 rounded-xl border px-3 py-5 text-center transition-colors duration-300'
            >
              <span className='flex size-8 items-center justify-center'>
                {getLobeIcon(provider.icon, 30)}
              </span>
              <span className='text-muted-foreground w-full truncate text-xs font-medium'>
                {provider.name}
              </span>
            </div>
          ))}

          <div className='border-border/50 bg-muted/30 text-muted-foreground flex flex-col items-center justify-center gap-2 rounded-xl border px-3 py-5 text-center'>
            <span className='text-base font-bold tracking-tight'>30+</span>
            <span className='w-full truncate text-xs font-medium'>
              {t('More providers')}
            </span>
          </div>
        </div>
      </div>
    </section>
  )
}
