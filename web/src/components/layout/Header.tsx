import { Bell, Search, User, ChevronDown } from 'lucide-react'
import { useState } from 'react'

export default function Header() {
  const [searchValue, setSearchValue] = useState('')

  return (
    <header className="h-16 sticky top-0 z-40 bg-[#f8f9fa]/80 backdrop-blur-sm border-b border-gray-200">
      <div className="h-full px-8 flex items-center justify-between">
        {/* Search */}
        <div className="relative w-80">
          <Search className="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
          <input
            type="text"
            value={searchValue}
            onChange={(e) => setSearchValue(e.target.value)}
            placeholder="搜索..."
            className="w-full h-10 pl-11 pr-4 bg-white rounded-xl text-sm text-gray-800 placeholder-gray-400 border border-gray-200 focus:outline-none focus:border-gray-300 transition-colors"
          />
        </div>

        {/* Right Actions */}
        <div className="flex items-center gap-3">
          {/* Notifications */}
          <button className="relative p-2.5 rounded-xl text-gray-500 hover:text-gray-900 hover:bg-gray-100 transition-colors">
            <Bell className="w-5 h-5" />
            <span className="absolute top-2 right-2 w-2 h-2 bg-red-500 rounded-full" />
          </button>

          {/* Divider */}
          <div className="w-px h-8 bg-gray-200" />

          {/* User Menu */}
          <button className="flex items-center gap-3 px-3 py-2 rounded-xl transition-colors hover:bg-gray-100">
            <div className="w-9 h-9 rounded-xl bg-gray-900 flex items-center justify-center">
              <User className="w-4 h-4 text-white" />
            </div>
            <div className="text-left">
              <p className="text-sm font-medium text-gray-900">Admin</p>
              <p className="text-xs text-gray-500">管理员</p>
            </div>
            <ChevronDown className="w-4 h-4 text-gray-400" />
          </button>
        </div>
      </div>
    </header>
  )
}
