// Package skills 技能系统：可插拔的 AI 技能注册与执行框架
package skills

import "sync"

// 全局技能注册表：通过 init 自动注册，RWMutex 保证并发安全
var (
	registry = make(map[string]*Skill) // 技能 ID → Skill 映射
	mu       sync.RWMutex              // 读写锁
)

// Register 注册技能到全局注册表
func Register(skill *Skill) {
	mu.Lock()
	defer mu.Unlock()
	registry[skill.ID] = skill
}

// Get 按 ID 获取技能
func Get(id string) *Skill {
	mu.RLock()
	defer mu.RUnlock()
	return registry[id]
}

// List 返回已启用的技能列表
func List() []*Skill {
	mu.RLock()
	defer mu.RUnlock()
	skills := make([]*Skill, 0, len(registry))
	for _, s := range registry {
		if s.Enabled {
			skills = append(skills, s)
		}
	}
	return skills
}
