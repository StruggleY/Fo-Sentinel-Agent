import { NavLink, useLocation } from 'react-router-dom'
import { useRef, useCallback, useEffect } from 'react'
import {
  LayoutDashboard,
  Rss,
  ShieldAlert,
  FileText,
  Settings,
  PanelLeftClose,
  Shield,
  MessageSquare,
  Cpu,
} from 'lucide-react'
import { useAppStore } from '@/stores/app'
import { useSettingsStore } from '@/stores/settingsStore'
import { cn } from '@/utils'

interface NavItem {
  path: string
  icon: typeof LayoutDashboard
  label: string
  highlight?: boolean
}

const navItems: NavItem[] = [
  { path: '/dashboard', icon: LayoutDashboard, label: '仪表盘' },
  { path: '/subscriptions', icon: Rss, label: '订阅管理' },
  { path: '/events', icon: ShieldAlert, label: '安全事件' },
  { path: '/events/analysis', icon: Cpu, label: 'Muiti-Agent 分析' },
  { path: '/chat', icon: MessageSquare, label: 'AI 助手'},
  { path: '/reports', icon: FileText, label: '分析报告' },
  { path: '/settings', icon: Settings, label: '系统设置' },
]

export default function Sidebar() {
  // @ts-ignore
  const { sidebarCollapsed, sidebarWidth, toggleSidebar, setSidebarWidth } = useAppStore()
  const siteName = useSettingsStore(s => s.siteName)
  const location = useLocation()
  const isResizing = useRef(false)
  const startX = useRef(0)
  const startWidth = useRef(0)

  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    isResizing.current = true
    startX.current = e.clientX
    startWidth.current = sidebarWidth
    document.body.style.cursor = 'col-resize'
    document.body.style.userSelect = 'none'
    e.preventDefault()
  }, [sidebarWidth])

  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      if (!isResizing.current) return
      const delta = e.clientX - startX.current
      const newWidth = Math.max(200, Math.min(370, startWidth.current + delta))
      setSidebarWidth(newWidth)
    }
    const handleMouseUp = () => {
      if (!isResizing.current) return
      isResizing.current = false
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
    }
    document.addEventListener('mousemove', handleMouseMove)
    document.addEventListener('mouseup', handleMouseUp)
    return () => {
      document.removeEventListener('mousemove', handleMouseMove)
      document.removeEventListener('mouseup', handleMouseUp)
    }
  }, [setSidebarWidth])

  return (
    <aside
      className="fixed left-0 top-0 h-screen bg-white z-50 flex flex-col border-r border-gray-200"
      style={{ width: sidebarCollapsed ? '72px' : `${sidebarWidth}px` }}
    >
      {/* Logo + 收起按钮（收起时仅显示按钮） */}
      <div
        className={cn(
          'h-16 border-b border-gray-100 flex-shrink-0',
          sidebarCollapsed ? 'flex items-center justify-center px-2' : 'grid grid-cols-[1fr_auto] items-center gap-3 pl-4 pr-2'
        )}
      >
        {!sidebarCollapsed && (
          <div className="flex items-center gap-2 min-w-0 overflow-hidden">
            <div className="w-8 h-8 rounded-lg bg-gray-900 flex items-center justify-center flex-shrink-0">
              <Shield className="w-4 h-4 text-white" />
            </div>
            <h1 className="text-lg font-semibold text-gray-900 tracking-tight min-w-0 truncate" title={siteName}>
              {siteName}
            </h1>
          </div>
        )}
        <button
          onClick={toggleSidebar}
          className="w-9 h-9 rounded-xl flex items-center justify-center text-gray-600 hover:bg-gray-100 hover:text-gray-900 transition-colors flex-shrink-0"
          title={sidebarCollapsed ? '打开边栏' : '收起边栏'}
        >
          <PanelLeftClose
            className={cn(
              'w-5 h-5 transition-transform duration-300',
              sidebarCollapsed && 'rotate-180'
            )}
          />
        </button>
      </div>

      {/* Navigation */}
      <nav className="flex-1 py-4 px-3 overflow-y-auto">
        <div className="space-y-1">
          {navItems.map((item) => {
            // 检查是否有其他路径是当前路径的子路径
            const hasSubPaths = navItems.some(other =>
              other.path !== item.path && other.path.startsWith(item.path + '/')
            )
            // 如果有子路径，只精确匹配；否则可以匹配子路径
            const isActive = location.pathname === item.path ||
              (!hasSubPaths && item.path !== '/dashboard' && location.pathname.startsWith(item.path + '/'))
            return (
              <NavLink
                key={item.path}
                to={item.path}
                className={cn(
                  'flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm font-medium transition-all duration-150',
                  isActive
                    ? item.highlight ? 'bg-primary-600 text-white' : 'bg-gray-900 text-white'
                    : item.highlight
                      ? 'text-primary-600 hover:text-primary-700 hover:bg-primary-50 bg-primary-50/60'
                      : 'text-gray-600 hover:text-gray-900 hover:bg-gray-100'
                )}
                title={sidebarCollapsed ? item.label : undefined}
              >
                <item.icon className="w-5 h-5 flex-shrink-0" />
                {!sidebarCollapsed && (
                  <span className="flex items-center gap-2 flex-1">
                    {item.label}
                    {item.highlight && !isActive && (
                      <span className="ml-auto text-[10px] font-semibold bg-primary-500 text-white px-1.5 py-0.5 rounded-full leading-none">
                        重点
                      </span>
                    )}
                  </span>
                )}
              </NavLink>
            )
          })}
        </div>
      </nav>

      {/* 拖拽手柄 */}
      {!sidebarCollapsed && (
        <div
          className="absolute right-0 top-0 h-full w-1.5 cursor-col-resize group z-10"
          onMouseDown={handleMouseDown}
        >
          <div className="absolute right-0 top-0 h-full w-px bg-gray-200 group-hover:bg-blue-400 group-hover:w-0.5 transition-all duration-150" />
        </div>
      )}
    </aside>
  )
}
