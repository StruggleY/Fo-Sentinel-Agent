import { NavLink, useLocation } from 'react-router-dom'
import { useRef, useCallback, useEffect, useState } from 'react'
import {
  LayoutDashboard,
  Rss,
  ShieldAlert,
  FileText,
  Settings,
  ChevronsLeft,
  ChevronsRight,
  Shield,
  MessageSquare,
  Cpu,
  BookMarked,
  Activity,
  LibraryBig,
  FlaskConical,
  ChevronDown,
} from 'lucide-react'
import { useAppStore } from '@/stores/app'
import { useSettingsStore } from '@/stores/settingsStore'
import { cn } from '@/utils'

interface NavItem {
  path: string
  icon: typeof LayoutDashboard
  label: string
  /** 精确匹配（有子路由时防止误激活父路由） */
  exact?: boolean
  /** 角标文字（如 NEW） */
  badge?: string
}

interface NavGroup {
  title: string
  /** 分组强调色（用于标题左侧色块） */
  accent: string
  /** 默认是否折叠（可选，适合次要分组） */
  defaultCollapsed?: boolean
  items: NavItem[]
}

const navGroups: NavGroup[] = [
  {
    title: 'AI 工作台',
    accent: '#818CF8',
    items: [
      { path: '/chat', icon: MessageSquare, label: 'RAG 智能对话' },
      { path: '/events/analysis', icon: Cpu, label: 'Multi-Agent 分析' },
      { path: '/knowledge', icon: LibraryBig, label: 'AI 知识库' },
    ],
  },
  {
    title: '安全运营',
    accent: '#34D399',
    items: [
      { path: '/dashboard', icon: LayoutDashboard, label: '仪表盘', exact: true },
      { path: '/subscriptions', icon: Rss, label: '订阅管理' },
      { path: '/events', icon: ShieldAlert, label: '安全事件', exact: true },
      { path: '/reports', icon: FileText, label: '分析报告' },
    ],
  },
  {
    title: '平台管理',
    accent: '#60A5FA',
    defaultCollapsed: false,
    items: [
      { path: '/rag-eval', icon: FlaskConical, label: 'RAG 质量评估' },
      { path: '/traces', icon: Activity, label: 'Agent Trace' },
      { path: '/term-mapping', icon: BookMarked, label: '术语规则' },
      { path: '/settings', icon: Settings, label: '系统设置' },
    ],
  },
]

export default function Sidebar() {
  // @ts-ignore
  const { sidebarCollapsed, sidebarWidth, toggleSidebar, setSidebarWidth } = useAppStore()
  const siteName = useSettingsStore(s => s.siteName)
  const location = useLocation()
  const isResizing = useRef(false)
  const startX = useRef(0)
  const startWidth = useRef(0)

  // 分组折叠状态：key = group.title，value = 是否折叠
  const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>(() => {
    const init: Record<string, boolean> = {}
    navGroups.forEach(g => {
      if (g.defaultCollapsed) init[g.title] = true
    })
    return init
  })

  const toggleGroup = (title: string) => {
    setCollapsedGroups(prev => ({ ...prev, [title]: !prev[title] }))
  }

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

  /** 判断导航项是否激活 */
  const isItemActive = (item: NavItem): boolean => {
    if (location.pathname === item.path) return true
    if (item.exact) return false
    return location.pathname.startsWith(item.path + '/')
  }

  /** 判断分组内是否有激活项（折叠时高亮分组标题） */
  const isGroupActive = (group: NavGroup): boolean =>
    group.items.some(item => isItemActive(item))

  return (
    <aside
      className="fixed left-0 top-0 h-screen z-50 flex flex-col transition-[width] duration-200"
      style={{
        width: sidebarCollapsed ? '72px' : `${sidebarWidth}px`,
        background: 'linear-gradient(180deg, #12172a 0%, #1a2035 50%, #1e2540 100%)',
        borderRight: '1px solid rgba(255,255,255,0.06)',
      }}
    >
      {/* Brand 区域 */}
      <div
        className={cn(
          'flex-shrink-0 pt-5 pb-3',
          sidebarCollapsed ? 'flex items-center justify-center px-3' : 'px-4',
        )}
      >
        {sidebarCollapsed ? (
          <div
            className="w-9 h-9 rounded-xl flex items-center justify-center flex-shrink-0"
            style={{ background: 'linear-gradient(135deg, #4F46E5, #7C3AED)' }}
          >
            <Shield className="w-4 h-4 text-white" />
          </div>
        ) : (
          <div className="flex items-center gap-3 min-w-0">
            <div
              className="w-9 h-9 rounded-xl flex items-center justify-center flex-shrink-0 shadow-lg"
              style={{ background: 'linear-gradient(135deg, #4F46E5, #7C3AED)' }}
            >
              <Shield className="w-4 h-4 text-white" />
            </div>
            <div className="min-w-0 overflow-hidden">
              <h1
                className="text-sm font-bold text-white truncate leading-tight"
                title={siteName}
              >
                {siteName}
              </h1>
              <p className="text-[11px] leading-tight mt-0.5" style={{ color: 'rgba(255,255,255,0.38)' }}>
                Security Console
              </p>
            </div>
          </div>
        )}
      </div>

      {/* 分割线 */}
      {!sidebarCollapsed && (
        <div className="mx-4 mb-2" style={{ height: '1px', background: 'rgba(255,255,255,0.06)' }} />
      )}

      {/* 导航 */}
      <nav className="flex-1 overflow-y-auto py-1 px-2 space-y-1 sidebar-scroll">
        {navGroups.map((group, groupIdx) => {
          const isGroupCollapsed = !!collapsedGroups[group.title]
          const groupHasActive = isGroupActive(group)

          return (
            <div key={group.title}>
              {/* 分组间距 */}
              {groupIdx > 0 && !sidebarCollapsed && (
                <div className="mx-2 my-2" style={{ height: '1px', background: 'rgba(255,255,255,0.05)' }} />
              )}
              {groupIdx > 0 && sidebarCollapsed && <div className="my-1" />}

              {/* 分组标题（展开状态） */}
              {!sidebarCollapsed && (
                <button
                  onClick={() => toggleGroup(group.title)}
                  className="w-full flex items-center justify-between px-3 py-1.5 rounded-md mb-0.5 transition-colors hover:bg-white/5 group"
                >
                  <div className="flex items-center gap-2">
                    <span
                      className="w-1.5 h-1.5 rounded-full flex-shrink-0"
                      style={{
                        background: group.accent,
                        opacity: groupHasActive ? 1 : 0.45,
                        boxShadow: groupHasActive ? `0 0 6px ${group.accent}` : 'none',
                      }}
                    />
                    <span
                      className="text-[10px] font-semibold tracking-widest uppercase"
                      style={{
                        color: groupHasActive && isGroupCollapsed
                          ? group.accent
                          : 'rgba(255,255,255,0.38)',
                      }}
                    >
                      {group.title}
                    </span>
                  </div>
                  <ChevronDown
                    className="w-3 h-3 transition-transform duration-200"
                    style={{
                      color: 'rgba(255,255,255,0.25)',
                      transform: isGroupCollapsed ? 'rotate(-90deg)' : 'rotate(0deg)',
                    }}
                  />
                </button>
              )}

              {/* 导航项列表（折叠时隐藏） */}
              {(!isGroupCollapsed || sidebarCollapsed) && (
                <div className="space-y-0.5">
                  {group.items.map(item => {
                    const active = isItemActive(item)
                    return (
                      <NavLink
                        key={item.path}
                        to={item.path}
                        title={sidebarCollapsed ? item.label : undefined}
                        className={cn(
                          'relative flex items-center gap-2.5 rounded-lg px-3 py-2 text-[13px] font-medium transition-all duration-150',
                          sidebarCollapsed && 'justify-center px-0',
                          active
                            ? 'text-white'
                            : 'hover:bg-white/[0.07] hover:text-white/90',
                        )}
                        style={{
                          background: active
                            ? `linear-gradient(90deg, rgba(99,102,241,0.22) 0%, rgba(99,102,241,0.08) 100%)`
                            : undefined,
                          color: active ? 'white' : 'rgba(255,255,255,0.58)',
                        }}
                      >
                        {/* 左侧激活指示条（使用分组 accent 色） */}
                        {active && !sidebarCollapsed && (
                          <span
                            className="absolute left-0 top-1.5 bottom-1.5 w-[3px] rounded-full"
                            style={{ background: group.accent }}
                          />
                        )}

                        <item.icon
                          className={cn('flex-shrink-0', sidebarCollapsed ? 'w-[18px] h-[18px]' : 'w-4 h-4')}
                          style={{ color: active ? group.accent : 'rgba(255,255,255,0.38)' }}
                        />

                        {!sidebarCollapsed && (
                          <>
                            <span className="flex-1 truncate">{item.label}</span>
                            {item.badge && (
                              <span
                                className="text-[9px] font-bold px-1.5 py-0.5 rounded-full leading-none"
                                style={{
                                  background: 'rgba(99,102,241,0.25)',
                                  color: '#A5B4FC',
                                  border: '1px solid rgba(99,102,241,0.3)',
                                }}
                              >
                                {item.badge}
                              </span>
                            )}
                          </>
                        )}
                      </NavLink>
                    )
                  })}
                </div>
              )}
            </div>
          )
        })}
      </nav>

      {/* 底部：折叠按钮 */}
      <div
        className="flex-shrink-0 px-3 pb-4 pt-2"
        style={{ borderTop: '1px solid rgba(255,255,255,0.05)' }}
      >
        {!sidebarCollapsed && (
          <p className="text-[10px] px-1 mb-2" style={{ color: 'rgba(255,255,255,0.2)' }}>
            v2.0
          </p>
        )}
        <button
          onClick={toggleSidebar}
          className="flex w-full items-center justify-center gap-2 rounded-lg py-2 text-xs transition-all hover:bg-white/[0.08]"
          style={{
            border: '1px solid rgba(255,255,255,0.09)',
            color: 'rgba(255,255,255,0.45)',
          }}
          title={sidebarCollapsed ? '展开侧边栏' : '收起侧边栏'}
        >
          {sidebarCollapsed ? (
            <ChevronsRight className="h-4 w-4" />
          ) : (
            <>
              <ChevronsLeft className="h-4 w-4" />
              <span>收起侧边栏</span>
            </>
          )}
        </button>
      </div>

      {/* 拖拽手柄 */}
      {!sidebarCollapsed && (
        <div
          className="absolute right-0 top-0 h-full w-1.5 cursor-col-resize group z-10"
          onMouseDown={handleMouseDown}
        >
          <div
            className="absolute right-0 top-0 h-full w-px transition-all duration-150 group-hover:w-[2px]"
            style={{ background: 'rgba(255,255,255,0.06)' }}
          />
        </div>
      )}
    </aside>
  )
}
