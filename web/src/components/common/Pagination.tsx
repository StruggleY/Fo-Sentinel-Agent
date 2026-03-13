import { useState } from 'react'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { cn } from '@/utils'

interface PaginationProps {
  page: number
  totalPages: number
  total: number
  onChange: (page: number) => void
}

export default function Pagination({ page, totalPages, total, onChange }: PaginationProps) {
  const [jumpInput, setJumpInput] = useState('')

  const handleJump = () => {
    const target = parseInt(jumpInput, 10)
    if (!isNaN(target) && target >= 1 && target <= totalPages) {
      onChange(target)
    }
    setJumpInput('')
  }

  if (totalPages <= 0) return null

  return (
    <div className="px-4 py-3 border-t border-gray-200 flex items-center justify-between flex-shrink-0 bg-white">
      {/* 左侧：总条数 */}
      <span className="text-sm text-gray-500">共 {total} 条</span>

      {/* 中间：分页控件 */}
      <div className="flex items-center gap-2">
        {/* 上一页 */}
        <button
          className={cn('pagination-item', page <= 1 && 'disabled')}
          onClick={() => onChange(Math.max(1, page - 1))}
          disabled={page <= 1}
        >
          <ChevronLeft className="w-4 h-4" />
        </button>

        {/* 页码信息 */}
        <span className="text-sm text-gray-600 px-2 whitespace-nowrap">
          第 <span className="font-medium text-gray-900">{page}</span> / <span className="font-medium text-gray-900">{totalPages}</span> 页
        </span>

        {/* 下一页 */}
        <button
          className={cn('pagination-item', page >= totalPages && 'disabled')}
          onClick={() => onChange(Math.min(totalPages, page + 1))}
          disabled={page >= totalPages}
        >
          <ChevronRight className="w-4 h-4" />
        </button>

        {/* 跳转 */}
        {totalPages > 1 && (
          <div className="flex items-center gap-1.5 ml-2 pl-2 border-l border-gray-200">
            <span className="text-sm text-gray-500">跳至</span>
            <input
              type="number"
              min={1}
              max={totalPages}
              value={jumpInput}
              onChange={e => setJumpInput(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && handleJump()}
              className="w-14 h-8 text-sm text-center border border-gray-300 rounded-lg focus:outline-none focus:ring-1 focus:ring-primary-400 focus:border-primary-400"
              placeholder={String(page)}
            />
            <button
              onClick={handleJump}
              className="h-8 px-2.5 text-sm rounded-lg border border-gray-300 text-gray-600 hover:bg-gray-50 transition-colors"
            >
              确定
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
