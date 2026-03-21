import React from 'react'
import { Shield, Search, FileText, ArrowRight } from 'lucide-react'
import { cn } from '@/utils'

// ── 静态预设开场卡片 ────────────────────────────────────────────────────────────

interface PresetCard {
  icon: React.ElementType
  iconBg: string
  iconColor: string
  title: string
  description: string
  question: string
}

const PRESET_CARDS: PresetCard[] = [
  {
    icon: Shield,
    iconBg: '#EFF6FF',
    iconColor: '#2563EB',
    title: '事件溯源分析',
    description: '研判告警根因与攻击链路',
    question: '分析最近 7 天 critical 级安全事件的根因、攻击路径和影响范围',
  },
  {
    icon: Search,
    iconBg: '#F5F3FF',
    iconColor: '#7C3AED',
    title: '威胁情报检索',
    description: '搜索最新漏洞与威胁动态',
    question: '联网查询最新 CVE 高危漏洞的利用情况、影响版本和修复建议',
  },
  {
    icon: FileText,
    iconBg: '#FFF7ED',
    iconColor: '#EA580C',
    title: '安全态势报告',
    description: '生成综合风险评估报告',
    question: '基于当前事件数据生成本周安全态势分析报告，包含风险评分和处置建议',
  },
]

// ── WelcomeScreen ─────────────────────────────────────────────────────────────

interface WelcomeScreenProps {
  onPresetSelect: (text: string) => void
  isLoading: boolean
  inputSlot?: React.ReactNode
}

export default function WelcomeScreen({ onPresetSelect, isLoading, inputSlot }: WelcomeScreenProps) {
  return (
    <div className="relative flex flex-col overflow-x-hidden min-h-full justify-center px-6 pb-8 pt-4">
      <div className="relative w-full max-w-[820px] mx-auto">

        {/* 标题区 */}
        <div className="text-center chat-welcome-anim" style={{ '--anim-delay': '0ms' } as React.CSSProperties}>
          <span className="inline-flex items-center gap-2 rounded-full border border-white/70 bg-white/80 px-3 py-1 text-xs font-medium text-[#2563EB] shadow-sm">
            <Shield className="h-3.5 w-3.5" />
            安全事件智能研判助手
          </span>
          <h1 className="mt-4 text-5xl font-bold leading-tight tracking-tight text-[#111827] sm:text-6xl">
            把威胁变成
            <span
              className="ml-2"
              style={{
                background: 'linear-gradient(135deg, #2563EB 0%, #7C3AED 50%, #2563EB 100%)',
                WebkitBackgroundClip: 'text',
                WebkitTextFillColor: 'transparent',
                backgroundClip: 'text',
              }}
            >
              清晰洞察
            </span>
          </h1>
          <p className="mt-4 text-base text-[#6B7280] sm:text-lg">
            安全事件研判、威胁情报分析，一次对话给出可操作建议
          </p>
        </div>

        {/* 输入框插槽 */}
        {inputSlot && (
          <div className="mt-8 chat-welcome-anim" style={{ '--anim-delay': '60ms' } as React.CSSProperties}>
            {inputSlot}
          </div>
        )}

        {/* 预设开场卡片 */}
        <div className="mt-8 chat-welcome-anim" style={{ '--anim-delay': '120ms' } as React.CSSProperties}>
          {/* 分隔标签 */}
          <div className="mb-5 flex items-center gap-3 text-xs uppercase tracking-[0.2em] text-[#94A3B8]">
            <span className="h-px flex-1 bg-gradient-to-r from-transparent to-[#E5E7EB]" />
            试试这些开场
            <span className="h-px flex-1 bg-gradient-to-l from-transparent to-[#E5E7EB]" />
          </div>

          {/* 三列卡片 */}
          <div className="grid gap-3 sm:grid-cols-3">
            {PRESET_CARDS.map((card) => {
              const Icon = card.icon
              return (
                <button
                  key={card.title}
                  type="button"
                  disabled={isLoading}
                  onClick={() => onPresetSelect(card.question)}
                  className={cn(
                    'group relative flex flex-col rounded-2xl border border-white/80 bg-white/80 p-4 text-left',
                    'shadow-sm transition-all duration-200',
                    'hover:border-[#BFDBFE] hover:bg-white hover:shadow-md',
                    isLoading && 'cursor-not-allowed opacity-60',
                  )}
                >
                  {/* 图标 + 标题 */}
                  <div className="flex items-center gap-3">
                    <span
                      className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl"
                      style={{ backgroundColor: card.iconBg }}
                    >
                      <Icon className="h-4 w-4" style={{ color: card.iconColor }} />
                    </span>
                    <div className="min-w-0">
                      <p className="text-sm font-semibold text-[#1F2937]">{card.title}</p>
                      <p className="text-xs text-[#6B7280]">{card.description}</p>
                    </div>
                  </div>

                  {/* 推荐问法 */}
                  <div className="mt-3 flex items-end justify-between gap-2">
                    <p className="line-clamp-2 text-xs leading-relaxed text-[#9CA3AF]">
                      推荐问法：{card.question}
                    </p>
                    <ArrowRight
                      className="h-3.5 w-3.5 flex-shrink-0 text-[#D1D5DB] transition-colors group-hover:text-[#3B82F6]"
                    />
                  </div>
                </button>
              )
            })}
          </div>
        </div>

      </div>
    </div>
  )
}
