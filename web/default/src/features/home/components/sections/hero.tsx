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
import type { ReactNode } from 'react'
import { Link } from '@tanstack/react-router'
import { BookOpen, KeyRound } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useStatus } from '@/hooks/use-status'
import { Button } from '@/components/ui/button'
import { ApiEndpointBar } from '../api-endpoint-bar'

interface HeroProps {
  className?: string
  isAuthenticated?: boolean
}

// Token color helpers for the decorative code block. Tailwind classes are used
// directly so the snippet stays self-contained in this file.
const TOK = {
  no: 'text-slate-400/55 dark:text-slate-500/40',
  method: 'text-blue-600/85 font-semibold dark:text-emerald-300/70',
  url: 'text-cyan-700/80 dark:text-sky-300/65',
  key: 'text-violet-700/85 dark:text-violet-300/70',
  string: 'text-emerald-700/80 dark:text-emerald-300/60',
  value: 'text-amber-700/85 dark:text-amber-300/65',
  punc: 'text-slate-500/65 dark:text-slate-400/45',
}

// 15 row decorative request/payload. Numbered, token-colored, purely visual.
// Rendered as JSX so each token can carry an independent color class without
// touching global CSS.
const CODE_ROWS: ReadonlyArray<ReactNode> = [
  <>
    <span className={TOK.method}>POST</span>{' '}
    <span className={TOK.url}>https://api.newapi.ai/v1/chat/completions</span>
  </>,
  <>
    <span className={TOK.key}>Content-Type</span>
    <span className={TOK.punc}>:</span>{' '}
    <span className={TOK.string}>application/json</span>
  </>,
  <>
    <span className={TOK.key}>Authorization</span>
    <span className={TOK.punc}>:</span>{' '}
    <span className={TOK.string}>Bearer sk-*****</span>
  </>,
  <>{' '}</>,
  <>
    <span className={TOK.punc}>{'{'}</span>
  </>,
  <>
    {'  '}
    <span className={TOK.key}>"model"</span>
    <span className={TOK.punc}>:</span>{' '}
    <span className={TOK.string}>"gpt-4o"</span>
    <span className={TOK.punc}>,</span>
  </>,
  <>
    {'  '}
    <span className={TOK.key}>"messages"</span>
    <span className={TOK.punc}>:</span>{' '}
    <span className={TOK.punc}>[</span>
  </>,
  <>
    {'    '}
    <span className={TOK.punc}>{'{'}</span>
  </>,
  <>
    {'      '}
    <span className={TOK.key}>"role"</span>
    <span className={TOK.punc}>:</span>{' '}
    <span className={TOK.string}>"user"</span>
    <span className={TOK.punc}>,</span>
  </>,
  <>
    {'      '}
    <span className={TOK.key}>"content"</span>
    <span className={TOK.punc}>:</span>{' '}
    <span className={TOK.string}>"Hello, who are you?"</span>
  </>,
  <>
    {'    '}
    <span className={TOK.punc}>{'}'}</span>
  </>,
  <>
    {'  '}
    <span className={TOK.punc}>],</span>
  </>,
  <>
    {'  '}
    <span className={TOK.key}>"stream"</span>
    <span className={TOK.punc}>:</span>{' '}
    <span className={TOK.value}>false</span>
    <span className={TOK.punc}>,</span>
  </>,
  <>
    {'  '}
    <span className={TOK.key}>"temperature"</span>
    <span className={TOK.punc}>:</span>{' '}
    <span className={TOK.value}>0.7</span>
  </>,
  <>
    <span className={TOK.punc}>{'}'}</span>
  </>,
]

// Scoped keyframes/utility classes for the hero title shimmer + sweep. Kept
// inline so we don't have to touch the global stylesheet for this one section.
const HERO_STYLE = `
@keyframes dh-hero-shimmer {
  0% { background-position: 0% 50%; }
  50% { background-position: 100% 50%; }
  100% { background-position: 0% 50%; }
}
@keyframes dh-hero-sweep {
  0% { background-position: -60% 0; opacity: 0; }
  15% { opacity: 1; }
  85% { opacity: 1; }
  100% { background-position: 160% 0; opacity: 0; }
}
.dh-hero-title-grad {
  background-image: linear-gradient(100deg,
    #2563eb 0%, #4f46e5 22%, #6366f1 38%, #c7d2fe 48%, #ffffff 50%,
    #c7d2fe 52%, #6366f1 62%, #7c3aed 78%, #8b5cf6 100%);
  background-size: 260% auto;
  background-repeat: no-repeat;
  background-position: 0% 50%;
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  color: transparent;
  animation: dh-hero-shimmer 7s ease-in-out 0.9s infinite;
}
.dark .dh-hero-title-grad {
  background-image: linear-gradient(100deg,
    #60a5fa 0%, #818cf8 22%, #a5b4fc 38%, #ddd6fe 48%, #ffffff 50%,
    #ddd6fe 52%, #a78bfa 62%, #c4b5fd 78%, #a78bfa 100%);
}
.dh-hero-sweep {
  position: absolute;
  left: 0;
  right: 0;
  bottom: -0.22em;
  height: 1px;
  pointer-events: none;
  background-image: linear-gradient(90deg,
    rgba(99,102,241,0) 0%, rgba(99,102,241,0) 35%,
    rgba(99,102,241,0.55) 50%,
    rgba(99,102,241,0) 65%, rgba(99,102,241,0) 100%);
  background-size: 220% 100%;
  background-position: -60% 0;
  background-repeat: no-repeat;
  opacity: 0;
  animation: dh-hero-sweep 6.5s ease-in-out 1.4s infinite;
}
.dark .dh-hero-sweep {
  background-image: linear-gradient(90deg,
    rgba(167,139,250,0) 0%, rgba(167,139,250,0) 35%,
    rgba(167,139,250,0.7) 50%,
    rgba(167,139,250,0) 65%, rgba(167,139,250,0) 100%);
}
@media (prefers-reduced-motion: reduce) {
  .dh-hero-title-grad { animation: none; }
  .dh-hero-sweep { display: none; }
}
`

export function Hero(props: HeroProps) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const docsUrl =
    (status?.docs_link as string | undefined) || 'https://docs.newapi.pro'
  const docsIsExternal = docsUrl.startsWith('http')

  const primaryHref = props.isAuthenticated ? '/keys' : '/sign-up'

  const renderDocsButton = () => {
    const inner = (
      <>
        <BookOpen data-icon='inline-start' />
        {t('Docs')}
      </>
    )
    if (docsIsExternal) {
      return (
        <Button
          variant='outline'
          size='lg'
          className='h-11 px-5 text-sm'
          render={
            <a href={docsUrl} target='_blank' rel='noopener noreferrer' />
          }
        >
          {inner}
        </Button>
      )
    }
    return (
      <Button
        variant='outline'
        size='lg'
        className='h-11 px-5 text-sm'
        render={<Link to={docsUrl} />}
      >
        {inner}
      </Button>
    )
  }

  return (
    <section className='relative z-10 overflow-hidden px-6 pt-28 pb-20 md:pt-36 md:pb-28'>
      {/* Scoped keyframes / classes used only by this hero. */}
      <style>{HERO_STYLE}</style>

      {/* Dark blue-black tech backdrop. Light mode keeps the same structure with
          brighter tokens; both layers are decorative and color-only. */}
      <div aria-hidden='true' className='pointer-events-none absolute inset-0 -z-10'>
        {/* Top radial wash */}
        <div
          className='absolute inset-0 opacity-30 dark:opacity-40'
          style={{
            background:
              'radial-gradient(ellipse 70% 55% at 50% 0%, var(--hero-glow, oklch(0.65 0.17 255 / 35%)) 0%, transparent 70%)',
          }}
        />
        {/* Perspective grid + fine dots, masked toward the bottom/right */}
        <div className='absolute inset-0 bg-[linear-gradient(to_right,var(--border)_1px,transparent_1px),linear-gradient(to_bottom,var(--border)_1px,transparent_1px)] bg-[size:3rem_3rem] opacity-[0.5] [mask-image:radial-gradient(ellipse_80%_70%_at_60%_40%,black_10%,transparent_85%)] dark:opacity-[0.35]' />
        <div className='absolute inset-0 bg-[radial-gradient(var(--border)_1px,transparent_1px)] bg-[size:1.5rem_1.5rem] opacity-[0.25] [mask-image:radial-gradient(ellipse_60%_60%_at_30%_70%,black_0%,transparent_75%)]' />

        {/* Decorative code block. 15 rows, line-numbered, token-colored, faded
            toward the right and the bottom so it never competes with the
            centered headline. Desktop-only to avoid mobile overflow. */}
        <div
          className='absolute top-32 left-4 hidden w-[26rem] font-mono text-[12.5px] leading-[1.7] tracking-tight select-none lg:block xl:left-10 2xl:left-16'
          style={{
            WebkitMaskImage:
              'linear-gradient(to right, black 0%, black 55%, transparent 100%), linear-gradient(to bottom, black 0%, black 60%, transparent 100%)',
            WebkitMaskComposite: 'source-in',
            maskImage:
              'linear-gradient(to right, black 0%, black 55%, transparent 100%), linear-gradient(to bottom, black 0%, black 60%, transparent 100%)',
            maskComposite: 'intersect',
          }}
        >
          {CODE_ROWS.map((row, i) => {
            const lineNo = String(i + 1).padStart(2, '0')
            return (
              <div key={lineNo} className='flex whitespace-pre'>
                <span className={`mr-4 w-6 shrink-0 text-right ${TOK.no}`}>
                  {lineNo}
                </span>
                <span className='min-w-0'>{row}</span>
              </div>
            )
          })}
        </div>
      </div>

      <div className='mx-auto flex max-w-5xl flex-col items-center text-center'>
        <h1
          className='landing-animate-fade-up relative max-w-3xl text-[clamp(2.25rem,7vw,3.5rem)] leading-[1.08] font-bold tracking-tight md:max-w-4xl md:text-[clamp(3rem,6.5vw,4.5rem)] lg:max-w-[58rem] lg:text-[5rem] lg:leading-[1.05]'
          style={{ animationDelay: '0ms' }}
        >
          <span className='block text-foreground'>{t('Unified')}</span>
          <span className='relative inline-block'>
            <span className='dh-hero-title-grad'>
              {t('Large-Model API Gateway')}
            </span>
            <span aria-hidden='true' className='dh-hero-sweep' />
          </span>
        </h1>

        <p
          className='landing-animate-fade-up text-muted-foreground mt-5 max-w-xl text-sm leading-relaxed opacity-0 md:text-base'
          style={{ animationDelay: '80ms' }}
        >
          {t(
            'One endpoint, compatible with the APIs of OpenAI, Claude, Gemini and other mainstream models.'
          )}
        </p>

        <div
          className='landing-animate-fade-up mt-8 w-full max-w-[880px] opacity-0'
          style={{ animationDelay: '140ms' }}
        >
          <ApiEndpointBar />
        </div>

        <div
          className='landing-animate-fade-up mt-7 flex flex-wrap items-center justify-center gap-3 opacity-0'
          style={{ animationDelay: '200ms' }}
        >
          <Button
            size='lg'
            className='h-11 px-5 text-sm'
            render={<Link to={primaryHref} />}
          >
            <KeyRound data-icon='inline-start' />
            {t('Get API Key')}
          </Button>
          {renderDocsButton()}
        </div>
      </div>
    </section>
  )
}
