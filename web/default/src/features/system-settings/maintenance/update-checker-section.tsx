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
import { useState } from 'react'
import { ExternalLinkIcon, RefreshCcwIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatTimestamp } from '@/lib/format'
import { api } from '@/lib/api'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { SettingsSection } from '../components/settings-section'

type DockerVersionData = {
  image: string
  tracking_tag: string
  current_version: string
  latest_version: string
  latest_revision: string
  latest_digest: string
  update_available: boolean
  package_url: string
}

type UpdateCheckerSectionProps = {
  currentVersion?: string | null
  startTime?: number | null
}

export function UpdateCheckerSection({
  currentVersion,
  startTime,
}: UpdateCheckerSectionProps) {
  const { t } = useTranslation()
  const [checking, setChecking] = useState(false)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [info, setInfo] = useState<DockerVersionData | null>(null)

  const uptime = startTime ? formatTimestamp(startTime) : t('Unknown')
  const version = currentVersion || t('Unknown')

  const handleCheckUpdates = async () => {
    setChecking(true)
    try {
      const response = await api.get('/api/status/docker-version', {
        skipBusinessError: true,
      })
      const payload = response.data
      if (!payload?.success || !payload?.data) {
        throw new Error(payload?.message || t('Failed to check for updates'))
      }
      const data = payload.data as DockerVersionData
      if (!data.update_available) {
        toast.success(
          t('You are running the latest version ({{version}}).', {
            version: data.latest_version || data.current_version,
          })
        )
        return
      }
      setInfo(data)
      setDialogOpen(true)
    } catch (error) {
      const message =
        error instanceof Error
          ? error.message
          : t('Failed to check for updates')
      toast.error(message)
    } finally {
      setChecking(false)
    }
  }

  const goToPackage = () => {
    if (info?.package_url) {
      window.open(info.package_url, '_blank', 'noopener,noreferrer')
    }
  }

  return (
    <>
      <SettingsSection title={t('System maintenance')}>
        <div className='space-y-6'>
          <div className='grid gap-4 md:grid-cols-2'>
            <div className='rounded-lg border p-4'>
              <div className='text-muted-foreground text-sm'>
                {t('Current version')}
              </div>
              <div className='text-lg font-semibold'>{version}</div>
            </div>
            <div className='rounded-lg border p-4'>
              <div className='text-muted-foreground text-sm'>
                {t('Uptime since')}
              </div>
              <div className='text-lg font-semibold'>{uptime}</div>
            </div>
          </div>

          <Button onClick={handleCheckUpdates} disabled={checking}>
            {checking ? (
              t('Checking updates...')
            ) : (
              <>
                <RefreshCcwIcon className='me-2 h-4 w-4' />
                {t('Check for updates')}
              </>
            )}
          </Button>
        </div>
      </SettingsSection>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className='max-h-[80vh] overflow-y-auto'>
          <DialogHeader>
            <DialogTitle>
              {info?.latest_version
                ? t('New version available: {{version}}', {
                    version: info.latest_version,
                  })
                : t('Release details')}
            </DialogTitle>
          </DialogHeader>

          <div className='space-y-3 text-sm'>
            <div className='grid grid-cols-[max-content_1fr] gap-x-4 gap-y-2'>
              <div className='text-muted-foreground'>{t('Image')}</div>
              <div className='break-all font-mono'>{info?.image}</div>
              <div className='text-muted-foreground'>{t('Tracking tag')}</div>
              <div className='font-mono'>{info?.tracking_tag}</div>
              <div className='text-muted-foreground'>
                {t('Current version')}
              </div>
              <div className='font-mono'>{info?.current_version}</div>
              <div className='text-muted-foreground'>
                {t('Latest version')}
              </div>
              <div className='font-mono'>{info?.latest_version}</div>
              {info?.latest_digest && (
                <>
                  <div className='text-muted-foreground'>{t('Digest')}</div>
                  <div className='break-all font-mono'>
                    {info.latest_digest}
                  </div>
                </>
              )}
              {info?.latest_revision && (
                <>
                  <div className='text-muted-foreground'>{t('Revision')}</div>
                  <div className='break-all font-mono'>
                    {info.latest_revision}
                  </div>
                </>
              )}
            </div>
          </div>

          <DialogFooter>
            <Button
              type='button'
              variant='secondary'
              onClick={() => setDialogOpen(false)}
            >
              {t('Close')}
            </Button>
            {info?.package_url && (
              <Button type='button' onClick={goToPackage}>
                <ExternalLinkIcon className='me-2 h-4 w-4' />
                {t('Open package page')}
              </Button>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
