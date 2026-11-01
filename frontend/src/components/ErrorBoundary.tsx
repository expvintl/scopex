import { Component, type ErrorInfo, type ReactNode } from 'react'
import { TriangleAlert } from 'lucide-react'
import { t } from '@/lib/i18n'

interface State {
  error: Error | null
}

export default class ErrorBoundary extends Component<{ children: ReactNode }, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error(t('渲染异常:'), error, info.componentStack)
  }

  render() {
    if (!this.state.error) return this.props.children
    return (
      <div className="flex h-full items-center justify-center p-6">
        <div className="panel max-w-md p-6 text-center">
          <TriangleAlert className="mx-auto h-7 w-7 text-warn" />
          <h2 className="mt-3 text-sm text-mist-200">{t("页面渲染出错")}</h2>
          <p className="mt-2 break-all font-mono text-[11px] text-mist-500">{this.state.error.message}</p>
          <button className="btn btn-primary mt-4 text-xs" onClick={() => window.location.reload()}>
            {t('重新加载')}
          </button>
        </div>
      </div>
    )
  }
}
