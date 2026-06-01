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

// Faint code snippet rendered on the left of the dark tech backdrop. Decorative
// only — kept out of the accessibility tree.
const CODE_SNIPPET = [
  'const client = new OpenAI({',
  '  baseURL: location.origin + "/v1",',
  '  apiKey: process.env.NEW_API_KEY,',
  '})',
  '',
  'const res = await client.chat.completions.create({',
  '  model: "your-model",',
  '  messages: [{ role: "user", content: "..." }],',
  '})',
]

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
        {/* Faint code snippet, left side, desktop only */}
        <pre className='text-muted-foreground/30 absolute top-32 -left-6 hidden max-w-md rotate-[-4deg] font-mono text-[11px] leading-relaxed select-none lg:block xl:left-4 dark:text-white/[0.06]'>
          {CODE_SNIPPET.join('\n')}
        </pre>
      </div>

      <div className='mx-auto flex max-w-3xl flex-col items-center text-center'>
        <h1
          className='landing-animate-fade-up text-[clamp(2rem,6vw,3.5rem)] leading-[1.12] font-bold tracking-tight'
          style={{ animationDelay: '0ms' }}
        >
          {t('Unified')}
          <br />
          <span className='bg-gradient-to-r from-blue-500 via-indigo-500 to-violet-500 bg-clip-text text-transparent dark:from-blue-400 dark:via-indigo-400 dark:to-violet-400'>
            {t('Large-Model API Gateway')}
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
          className='landing-animate-fade-up mt-8 w-full max-w-xl opacity-0'
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
