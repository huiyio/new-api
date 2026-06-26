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
import type { CSSProperties } from 'react'
import { useTranslation } from 'react-i18next'
import { getLobeIcon } from '@/lib/lobe-icon'

interface ProviderGridProps {
  className?: string
}

interface FeaturedProvider {
  name: string
  icon: string
  pillKey: string
  accent: string
  dot: string
  brandRgb: string
  pillClasses: string
}

interface SecondaryProvider {
  name: string
  icon: string
  pillKey: string
}

const FEATURED: FeaturedProvider[] = [
  {
    name: 'OpenAI',
    icon: 'OpenAI',
    pillKey: 'OpenAI API compatible',
    accent: 'from-emerald-400/0 via-emerald-400/70 to-emerald-400/0',
    dot: 'bg-emerald-500 dark:bg-emerald-400',
    brandRgb: '52, 211, 153',
    pillClasses:
      'border-emerald-400/40 bg-emerald-400/10 text-emerald-700 dark:border-emerald-400/30 dark:bg-emerald-400/10 dark:text-emerald-300',
  },
  {
    name: 'Claude',
    icon: 'Claude.Color',
    pillKey: 'Anthropic API compatible',
    accent: 'from-orange-400/0 via-orange-400/70 to-orange-400/0',
    dot: 'bg-orange-500 dark:bg-orange-400',
    brandRgb: '251, 146, 60',
    pillClasses:
      'border-orange-400/40 bg-orange-400/10 text-orange-700 dark:border-orange-400/30 dark:bg-orange-400/10 dark:text-orange-300',
  },
  {
    name: 'Google Gemini',
    icon: 'Gemini.Color',
    pillKey: 'Gemini API compatible',
    accent: 'from-sky-400/0 via-sky-400/70 to-sky-400/0',
    dot: 'bg-sky-500 dark:bg-sky-400',
    brandRgb: '56, 189, 248',
    pillClasses:
      'border-sky-400/40 bg-sky-400/10 text-sky-700 dark:border-sky-400/30 dark:bg-sky-400/10 dark:text-sky-300',
  },
  {
    name: 'DeepSeek',
    icon: 'DeepSeek.Color',
    pillKey: 'DeepSeek API compatible',
    accent: 'from-indigo-400/0 via-indigo-400/70 to-indigo-400/0',
    dot: 'bg-indigo-500 dark:bg-indigo-400',
    brandRgb: '129, 140, 248',
    pillClasses:
      'border-indigo-400/40 bg-indigo-400/10 text-indigo-700 dark:border-indigo-400/30 dark:bg-indigo-400/10 dark:text-indigo-300',
  },
]

const SECONDARY: SecondaryProvider[] = [
  { name: 'Meta Llama', icon: 'Meta.Color', pillKey: 'OpenAI API compatible' },
  {
    name: 'Mistral AI',
    icon: 'Mistral.Color',
    pillKey: 'OpenAI API compatible',
  },
  { name: 'xAI Grok', icon: 'Grok', pillKey: 'OpenAI API compatible' },
  {
    name: 'Azure OpenAI',
    icon: 'Azure.Color',
    pillKey: 'OpenAI API compatible',
  },
  { name: 'Moonshot AI', icon: 'Moonshot', pillKey: 'OpenAI API compatible' },
  { name: 'Qwen', icon: 'Qwen.Color', pillKey: 'OpenAI API compatible' },
  { name: 'Yi', icon: 'Yi.Color', pillKey: 'OpenAI API compatible' },
  { name: 'MiniMax', icon: 'Minimax.Color', pillKey: 'OpenAI API compatible' },
  { name: 'StepFun', icon: 'Stepfun.Color', pillKey: 'OpenAI API compatible' },
  {
    name: 'Baichuan',
    icon: 'Baichuan.Color',
    pillKey: 'OpenAI API compatible',
  },
  { name: '360 智脑', icon: 'Ai360.Color', pillKey: 'OpenAI API compatible' },
]

// Scoped CSS for the provider grid. Provides:
//   - staggered fade-up entrance (heading + cards)
//   - featured card hover: translateY(-3px) + brand-color shadow
//   - featured icon hover: scale(1.06) + brand-color drop-shadow
//   - secondary card hover: translateY(-2px) + softer shadow
//   - secondary icon hover: scale(1.08)
//   - diagonal hover sweep light via ::after (not a persistent halo)
//   - prefers-reduced-motion disables entrance/hover transforms safely
// No always-on halo, no dashed spinning ring, no continuous breathing icon.
const PG_STYLE = `
@keyframes np-provider-fade-up {
  from {
    opacity: 0;
    transform: translate3d(0, 18px, 0);
  }
  to {
    opacity: 1;
    transform: translate3d(0, 0, 0);
  }
}
@keyframes np-provider-hover-sweep {
  0% { opacity: 0; transform: translateX(0) skewX(-15deg); }
  12% { opacity: 1; }
  100% { opacity: 0; transform: translateX(300%) skewX(-15deg); }
}
/* 使用 backwards 填充：延迟期间停留在 from 关键帧（透明 + 下移），
   动画结束后释放 transform/opacity 占用，让卡片 hover translateY 能生效。
   不要改成 both / forwards，否则 to 关键帧的 transform 会持续覆盖 :hover。 */
.np-provider-fade-item {
  animation: np-provider-fade-up 0.6s cubic-bezier(0.22, 1, 0.36, 1) backwards;
  will-change: opacity, transform;
}
.np-provider-card-featured {
  transition: transform 0.35s ease, box-shadow 0.35s ease;
}
.np-provider-card-featured:hover {
  transform: translateY(-3px);
  box-shadow:
    0 14px 30px -10px rgba(var(--np-provider-brand, 100, 116, 139), 0.35),
    0 4px 10px -4px rgba(15, 23, 42, 0.12);
}
.np-provider-featured-icon {
  transition: transform 0.4s ease, filter 0.4s ease;
  transform-origin: center;
  color: rgb(var(--np-provider-brand, 100, 116, 139));
}
.np-provider-card-featured:hover .np-provider-featured-icon {
  transform: scale(1.06);
  filter: drop-shadow(0 6px 14px rgba(var(--np-provider-brand, 100, 116, 139), 0.5));
}
.np-provider-card-secondary {
  position: relative;
  transition: transform 0.3s ease, box-shadow 0.3s ease,
    border-color 0.3s ease, background-color 0.3s ease;
}
.np-provider-card-secondary:hover {
  transform: translateY(-2px);
  box-shadow: 0 10px 22px -10px rgba(15, 23, 42, 0.18);
}
.dark .np-provider-card-secondary:hover {
  box-shadow: 0 10px 22px -10px rgba(0, 0, 0, 0.5);
}
.np-provider-secondary-icon {
  transition: transform 0.3s ease;
  transform-origin: center;
}
.np-provider-card-secondary:hover .np-provider-secondary-icon {
  transform: scale(1.08);
}
.np-provider-card-featured::after {
  content: '';
  position: absolute;
  top: 0;
  bottom: 0;
  left: -45%;
  width: 42%;
  transform: skewX(-15deg);
  background: linear-gradient(
    100deg,
    rgba(var(--np-provider-brand, 100, 116, 139), 0) 0%,
    rgba(var(--np-provider-brand, 100, 116, 139), 0.28) 45%,
    rgba(255, 255, 255, 0.42) 55%,
    rgba(var(--np-provider-brand, 100, 116, 139), 0) 100%
  );
  pointer-events: none;
  z-index: 2;
  opacity: 0;
  will-change: transform, opacity;
}
.np-provider-card-featured:hover::after {
  animation: np-provider-hover-sweep 0.78s ease-out both;
}
.np-provider-card-secondary::after {
  content: '';
  position: absolute;
  top: 0;
  bottom: 0;
  left: -45%;
  width: 42%;
  transform: skewX(-15deg);
  background: linear-gradient(
    100deg,
    rgba(59, 130, 246, 0) 0%,
    rgba(59, 130, 246, 0.24) 45%,
    rgba(255, 255, 255, 0.36) 55%,
    rgba(59, 130, 246, 0) 100%
  );
  pointer-events: none;
  z-index: 2;
  opacity: 0;
  will-change: transform, opacity;
}
.np-provider-card-secondary:hover::after {
  animation: np-provider-hover-sweep 0.78s ease-out both;
}
.dark .np-provider-card-featured::after {
  background: linear-gradient(
    100deg,
    rgba(var(--np-provider-brand, 100, 116, 139), 0) 0%,
    rgba(var(--np-provider-brand, 100, 116, 139), 0.18) 45%,
    rgba(255, 255, 255, 0.22) 55%,
    rgba(var(--np-provider-brand, 100, 116, 139), 0) 100%
  );
}
.dark .np-provider-card-secondary::after {
  background: linear-gradient(
    100deg,
    rgba(59, 130, 246, 0) 0%,
    rgba(59, 130, 246, 0.15) 45%,
    rgba(255, 255, 255, 0.20) 55%,
    rgba(59, 130, 246, 0) 100%
  );
}
@media (prefers-reduced-motion: reduce) {
  .np-provider-fade-item {
    opacity: 1;
    animation: none;
  }
  .np-provider-card-featured,
  .np-provider-card-secondary,
  .np-provider-featured-icon,
  .np-provider-secondary-icon {
    transition: none;
  }
  .np-provider-card-featured:hover,
  .np-provider-card-secondary:hover,
  .np-provider-card-featured:hover .np-provider-featured-icon,
  .np-provider-card-secondary:hover .np-provider-secondary-icon {
    transform: none;
    filter: none;
    box-shadow: none;
  }
}
`

/**
 * Provider showcase. Featured row: four wide white-glass cards with a colored
 * top hairline and a centered icon/name/compatibility pill. Secondary row:
 * compact rectangular cards laid out 6×2 on desktop, with a leading icon and
 * a small dot+text compatibility pill on the right.
 */
export function ProviderGrid(props: ProviderGridProps) {
  const { t } = useTranslation()

  return (
    <section className={props.className}>
      {/* Scoped keyframes / hover styles used only by this grid. */}
      <style>{PG_STYLE}</style>
      <div className='mx-auto w-full max-w-[1280px]'>
        <div className='text-center'>
          <h2
            className='np-provider-fade-item text-xl font-bold tracking-tight md:text-2xl'
            style={{ animationDelay: '0ms' }}
          >
            {t('Supports a vast range of large-model providers')}
          </h2>
          <p
            className='np-provider-fade-item text-muted-foreground mx-auto mt-2 max-w-xl text-sm'
            style={{ animationDelay: '70ms' }}
          >
            {t('One API to access all mainstream large-model providers')}
          </p>
        </div>

        <div className='mt-6 grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4'>
          {FEATURED.map((provider, index) => (
            <div
              key={provider.name}
              className='np-provider-card-featured np-provider-fade-item bg-card dark:border-border/60 group relative flex min-h-[220px] flex-col overflow-hidden rounded-[8px] border border-[rgba(15,23,42,0.24)] px-4 py-8 shadow-[0_12px_30px_-16px_rgba(15,23,42,0.42),0_4px_12px_rgba(15,23,42,0.13)] lg:h-[288px]'
              style={
                {
                  animationDelay: `${140 + index * 70}ms`,
                  '--np-provider-brand': provider.brandRgb,
                  '--np-provider-brand-soft': `rgba(${provider.brandRgb}, 0.12)`,
                  '--np-provider-brand-border': `rgba(${provider.brandRgb}, 0.4)`,
                } as CSSProperties
              }
            >
              <span
                aria-hidden='true'
                className={`absolute inset-x-0 top-0 h-[2px] bg-gradient-to-r ${provider.accent}`}
              />
              <div className='relative flex flex-1 flex-col items-center justify-center gap-3.5 text-center'>
                <span className='np-provider-featured-icon relative flex size-[88px] items-center justify-center'>
                  {getLobeIcon(provider.icon, 72)}
                </span>
                <span className='w-full truncate text-xl font-semibold tracking-tight'>
                  {provider.name}
                </span>
                <span
                  className={`${provider.pillClasses} inline-flex max-w-full items-center gap-1.5 rounded-full border px-3 py-1 text-xs`}
                >
                  <span
                    aria-hidden='true'
                    className={`size-1.5 shrink-0 rounded-full ${provider.dot}`}
                  />
                  <span className='truncate'>{t(provider.pillKey)}</span>
                </span>
              </div>
            </div>
          ))}
        </div>

        <div className='mt-4 grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-6'>
          {SECONDARY.map((provider, index) => (
            <div
              key={provider.name}
              className='np-provider-card-secondary np-provider-fade-item bg-card dark:border-border/60 hover:border-border hover:bg-card flex min-h-[84px] items-center gap-2.5 overflow-hidden rounded-[8px] border border-[rgba(15,23,42,0.22)] px-3 py-2.5 shadow-[0_9px_22px_-14px_rgba(15,23,42,0.38),0_3px_9px_rgba(15,23,42,0.12)] lg:h-[92px]'
              style={{ animationDelay: `${420 + index * 40}ms` }}
            >
              <span className='np-provider-secondary-icon flex size-8 shrink-0 items-center justify-center'>
                {getLobeIcon(provider.icon, 28)}
              </span>
              <div className='flex min-w-0 flex-1 flex-col gap-1'>
                <span className='truncate text-sm font-semibold tracking-tight'>
                  {provider.name}
                </span>
                <span className='text-muted-foreground inline-flex w-fit max-w-full items-center gap-1.5 rounded-full border border-transparent px-0 py-0.5 text-[11px] leading-[1.4]'>
                  <span
                    aria-hidden='true'
                    className='size-1 shrink-0 rounded-full bg-current opacity-[0.55]'
                  />
                  <span className='truncate'>{t(provider.pillKey)}</span>
                </span>
              </div>
            </div>
          ))}

          <div
            className='np-provider-card-secondary np-provider-fade-item dark:border-border/60 bg-muted/30 text-muted-foreground flex min-h-[84px] items-center gap-2.5 overflow-hidden rounded-[8px] border border-[rgba(15,23,42,0.22)] px-3 py-2.5 shadow-[0_9px_22px_-14px_rgba(15,23,42,0.38),0_3px_9px_rgba(15,23,42,0.12)] lg:h-[92px]'
            style={{ animationDelay: `${420 + SECONDARY.length * 40}ms` }}
          >
            <span className='np-provider-secondary-icon flex size-8 shrink-0 items-center justify-center text-xs font-bold tracking-tight'>
              30+
            </span>
            <div className='flex min-w-0 flex-1 flex-col gap-1'>
              <span className='truncate text-sm font-semibold tracking-tight'>
                {t('More providers')}
              </span>
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
