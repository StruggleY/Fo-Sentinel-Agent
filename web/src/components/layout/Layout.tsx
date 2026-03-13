import { Outlet } from 'react-router-dom'
import Sidebar from './Sidebar'
import Header from './Header'
import { useAppStore } from '@/stores/app'
import ReportGenerateModal from '@/components/ReportGenerateModal'

export default function Layout() {
  const { sidebarCollapsed, sidebarWidth } = useAppStore()

  return (
    <div className="min-h-screen bg-[#f8f9fa]">
      <Sidebar />
      <div
        className="transition-[margin] duration-300"
        style={{ marginLeft: sidebarCollapsed ? '72px' : `${sidebarWidth}px` }}
      >
        <Header />
        <main className="p-8">
          <Outlet />
        </main>
      </div>
      {/* 报告生成进度弹窗：fixed 定位，切换页面后持续显示 */}
      <ReportGenerateModal />
    </div>
  )
}
