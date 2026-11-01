import { Languages } from 'lucide-react'

import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useI18n, type LangPref } from '@/lib/i18n'

export function LangToggle() {
  const { pref, setPref, t } = useI18n()
  const label = pref === 'auto' ? t('自动检测') : pref === 'zh' ? '中文' : 'English'

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="sm" className="gap-1.5 px-2 text-muted-foreground" aria-label={t('切换语言')}>
          <Languages className="size-4" />
          <span className="hidden text-xs sm:inline">{label}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-40">
        <DropdownMenuLabel>{t('语言')}</DropdownMenuLabel>
        <DropdownMenuRadioGroup value={pref} onValueChange={(value) => setPref(value as LangPref)}>
          <DropdownMenuRadioItem value="auto">{t('自动检测')}</DropdownMenuRadioItem>
          <DropdownMenuRadioItem value="zh">中文</DropdownMenuRadioItem>
          <DropdownMenuRadioItem value="en">English</DropdownMenuRadioItem>
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
