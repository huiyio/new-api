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
import {
  Network,
  Plug,
  Gauge,
  ShieldCheck,
  Wallet,
  type LucideIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

interface FeatureStripProps {
  className?: string
}

interface StripItem {
  icon: LucideIcon
  title: string
  desc: string
  iconClassName: string
}

/**
 * First-screen feature strip: five short value props with lucide icons.
 * Subtle accent colors (cyan/green/amber) keep the row from reading as a single
 * blue/violet block; colors are decorative only and applied via icon className.
 */
export function FeatureStrip(props: FeatureStripProps) {
  const { t } = useTranslation()

  const items: StripItem[] = [
    {
      icon: Network,
      title: t('Unified Access'),
      desc: t('One endpoint for 40+ upstream providers'),
      iconClassName: 'text-blue-500 dark:text-blue-400',
    },
    {
      icon: Plug,
      title: t('OpenAI API Compatible'),
      desc: t('Drop-in compatible with mainstream SDKs'),
      iconClassName: 'text-cyan-500 dark:text-cyan-400',
    },
    {
      icon: Gauge,
      title: t('Fast & Stable'),
      desc: t('Low-latency routing with load balancing'),
      iconClassName: 'text-emerald-500 dark:text-emerald-400',
    },
    {
      icon: ShieldCheck,
      title: t('Secure & Reliable'),
      desc: t('Fine-grained keys and access control'),
      iconClassName: 'text-violet-500 dark:text-violet-400',
    },
    {
      icon: Wallet,
      title: t('Cost-Effective'),
      desc: t('Transparent, pay-as-you-go billing'),
      iconClassName: 'text-amber-500 dark:text-amber-400',
    },
  ]

  return (
    <section className={props.className}>
      <div className='border-border/40 bg-muted/10 mx-auto max-w-6xl rounded-2xl border px-4 py-6 sm:px-6'>
        <div className='grid grid-cols-2 gap-x-4 gap-y-6 sm:grid-cols-3 lg:grid-cols-5'>
          {items.map((item) => {
            const Icon = item.icon
            return (
              <div key={item.title} className='flex flex-col gap-1.5'>
                <span className='border-border/50 bg-card/60 flex size-9 items-center justify-center rounded-lg border'>
                  <Icon className={`size-4 ${item.iconClassName}`} />
                </span>
                <span className='text-sm font-semibold tracking-tight'>
                  {item.title}
                </span>
                <span className='text-muted-foreground text-xs leading-relaxed'>
                  {item.desc}
                </span>
              </div>
            )
          })}
        </div>
      </div>
    </section>
  )
}
