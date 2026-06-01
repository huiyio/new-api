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
import { Check, ChevronDown, Copy } from 'lucide-react'
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
        'border-border/60 bg-card/70 flex w-full items-center gap-2 rounded-xl border p-1.5 pl-3 shadow-xs backdrop-blur-sm sm:gap-3 sm:pl-4',
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
        <ChevronDown
          aria-hidden='true'
          className='text-muted-foreground/50 size-3.5'
        />
      </div>

      {/* Endpoint URL */}
      <code className='text-foreground/80 min-w-0 flex-1 truncate font-mono text-[12px] sm:text-[13px]'>
        <span className='text-muted-foreground/70'>{baseUrl}</span>
        <span className='text-foreground'>{ENDPOINT_PATH}</span>
      </code>

      {/* Copy action */}
      <Button
        type='button'
        variant='ghost'
        size='icon-sm'
        className='shrink-0'
        onClick={() => copyToClipboard(fullEndpoint)}
        aria-label={isCopied ? t('Copied') : t('Copy')}
      >
        {isCopied ? <Check className='text-success' /> : <Copy />}
      </Button>
    </div>
  )
}
