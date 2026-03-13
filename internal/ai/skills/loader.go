// Package skills 技能系统：可插拔的 AI 技能注册与执行框架
package skills

import (
	"context"
	"os"
	"path/filepath"

	"github.com/gogf/gf/v2/frame/g"
)

// skillNameMap 技能 ID 到中文名称的映射
var skillNameMap = map[string]string{
	"event-analysis": "事件分析",
	"threat-hunting": "威胁狩猎",
	"log-diagnosis":  "日志诊断",
}

// LoadAllSkills 扫描 skills/ 目录，加载所有 SKILL.md 技能
func LoadAllSkills(skillsDir string) error {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return err
	}

	ctx := context.Background()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
		if _, err := os.Stat(skillPath); os.IsNotExist(err) {
			continue
		}

		// 【文件读取】读取完整文件
		content, err := os.ReadFile(skillPath)
		if err != nil {
			g.Log().Warningf(ctx, "failed to read skills %s: %v", entry.Name(), err)
			continue
		}

		// 【完整解析】直接解析完整内容（frontmatter + body）
		skill, err := ParseSkillMD(content)
		if err != nil {
			g.Log().Warningf(ctx, "failed to parse skills %s: %v", entry.Name(), err)
			continue
		}

		// 获取中文名称
		displayName := skill.Name
		if cnName, ok := skillNameMap[skill.ID]; ok {
			displayName = cnName
		}
		skill.Name = displayName

		Register(skill)
		g.Log().Infof(ctx, "loaded skills: %s", skill.ID)
	}

	return nil
}
