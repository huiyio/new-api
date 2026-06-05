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
import { useMemo } from 'react'
import { Check, Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Button } from '@/components/ui/button'

const ENDPOINT_PATH = '/v1/chat/completions'

interface ApiEndpointBarProps {
  className?: string
}

/**
 * Hero API endpoint strip: status dot + POST method, the resolved base URL and
 * chat-completions path, and a copy-to-clipboard action with perceivable feedback.
 *
 * Visual target: a ~46px tall pill with rounded corners, a faint blue/violet
 * glow ring, a vertical divider separating the POST block from the URL, and
 * the request path rendered in a primary accent color.
 */
export function ApiEndpointBar(props: ApiEndpointBarProps) {
  const { t } = useTranslation()
  const { copiedText, copyToClipboard } = useCopyToClipboard()

  const baseUrl = useMemo(() => {
    if (typeof window !== 'undefined' && window.location?.origin) {
      return window.location.origin
    }
    return 'https://your-domain.com'
  }, [])

  const fullEndpoint = `${baseUrl}${ENDPOINT_PATH}`
  const isCopied = copiedText === fullEndpoint

  return (
    <div
      className={cn(
        'border-border/60 bg-card/70 group relative flex h-[46px] w-full items-center gap-2 rounded-[10px] border pr-1 pl-3 backdrop-blur-sm transition-shadow sm:gap-3 sm:pl-4',
        // Subtle outer glow + 1px shadow; deepens slightly on hover. Tuned for
        // both light and dark so the strip reads as a soft, lit panel.
        'shadow-[0_1px_2px_rgba(15,23,42,0.05),0_0_18px_-10px_rgba(99,102,241,0.55)]',
        'hover:shadow-[0_1px_2px_rgba(15,23,42,0.06),0_0_22px_-8px_rgba(99,102,241,0.7)]',
        'dark:bg-white/[0.04]',
        'dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.04),0_0_26px_-6px_rgba(129,140,248,0.55)]',
        'dark:hover:shadow-[inset_0_1px_0_rgba(255,255,255,0.06),0_0_32px_-4px_rgba(129,140,248,0.7)]',
        props.className
      )}
    >
      {/* Status dot + method */}
      <div className='flex shrink-0 items-center gap-2'>
        <span className='relative flex size-2'>
          <span className='absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-70' />
          <span className='relative inline-flex size-2 rounded-full bg-emerald-500' />
        </span>
        <span className='font-mono text-[11px] font-semibold tracking-wider text-emerald-600 sm:text-xs dark:text-emerald-400'>
          POST
        </span>
      </div>

      {/* Vertical divider separating the method block from the URL. */}
      <span
        aria-hidden='true'
        className='bg-border/70 h-5 w-px shrink-0 dark:bg-white/10'
      />

      {/* Endpoint URL — base host muted, path in the primary accent so the
          relevant part of the request stays visually anchored. */}
      <code className='min-w-0 flex-1 truncate text-left font-mono text-[12px] sm:text-[13px]'>
        <span className='text-muted-foreground/80'>{baseUrl}</span>
        <span className='font-medium text-indigo-600 dark:text-indigo-300'>
          {ENDPOINT_PATH}
        </span>
      </code>

      {/* Copy action */}
      <Button
        type='button'
        variant='ghost'
        size='icon-sm'
        className='mr-1 shrink-0'
        onClick={() => copyToClipboard(fullEndpoint)}
        aria-label={isCopied ? t('Copied') : t('Copy')}
      >
        {isCopied ? <Check className='text-success' /> : <Copy />}
      </Button>
    </div>
  )
}
