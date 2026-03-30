package tasks

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/database"
	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/utils"
)

// regex patterns for script comment parsing
var (
	envRegex  = regexp.MustCompile(`(?i)[new ]*Env\(['"]?([^'"]+)['"]?\)[;]?`)
	cronRegex = regexp.MustCompile(`(?i)(?:cron[ \t]*[:=]?[ \t]*['"]([^'"]+)['"])|(?:(?:^|[ \t\*\/])([0-9\*\/\-,L?]+[ \t]+[0-9\*\/\-,L?#]+[ \t]+[0-9\*\/\-,L?#]+[ \t]+[0-9\*\/\-,L?#]+[ \t]+[0-9\*\/\-,L?#]+(?:[ \t]+[0-9\*\/\-,L?#]+)?))`)
)

// ParseRepoScriptsAndAddCron scans the repo dir for scripts, parses cron and env comments, and registers tasks
func ParseRepoScriptsAndAddCron(es *ExecutorService, taskID string, logWriter io.Writer) {
	// help print logs to writer if provided
	log := func(format string, a ...interface{}) {
		msg := fmt.Sprintf(format, a...)
		if !strings.HasSuffix(msg, "\n") {
			msg += "\n"
		}
		if logWriter != nil {
			logWriter.Write([]byte(msg))
		}
		// logger.Info(msg)
	}

	var repoTask models.Task
	res := database.DB.Where("id = ?", taskID).Limit(1).Find(&repoTask)
	if res.Error != nil || res.RowsAffected == 0 {
		return
	}

	if repoTask.Type != constant.TaskTypeRepo {
		return
	}

	var repoCfg models.RepoConfig
	if err := json.Unmarshal([]byte(repoTask.Config), &repoCfg); err != nil {
		return
	}

	if repoCfg.RepoSource != "ql" || !repoCfg.AutoAddCron {
		return
	}

	// target path
	targetPath := repoCfg.TargetPath
	if targetPath == "" {
		targetPath = repoTask.WorkDir
	} else if !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(resolveAbsScriptsDir(), targetPath)
	}
	if targetPath == "" {
		return
	}
	targetPath = filepath.Clean(targetPath)

	// We might have appended a repo id to targetPath
	repoId := utils.GetRepoIdentifier(repoCfg.SourceURL, repoCfg.Branch)
	
	gitDir := filepath.Join(targetPath, ".git")
	if !isDir(targetPath) || !pathExists(gitDir) {
		repoPath := filepath.Join(targetPath, repoId)
		if pathExists(repoPath) {
			targetPath = repoPath
		}
	}

	if !pathExists(targetPath) {
		return
	}

	// tag used during sync
	tag := fmt.Sprintf("%s", repoId)

	exts := []string{".js", ".py", ".ts", ".sh", ".php"}
	if repoCfg.Extensions != "" {
		customExts := splitKeywords(repoCfg.Extensions)
		if len(customExts) > 0 {
			exts = nil
			for _, e := range customExts {
				e = strings.TrimSpace(e)
				if e != "" {
					if !strings.HasPrefix(e, ".") {
						e = "." + e
					}
					exts = append(exts, e)
				}
			}
		}
	}

	log("\n----------------------------------------")
	log("  开始扫描脚本并自动注册定时任务  ")
	log("----------------------------------------")

	foundSourceIDs := make(map[string]bool)
	newTaskCount := 0
	updateTaskCount := 0

	filepath.WalkDir(targetPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		if strings.Contains(path, ".git") {
			return nil
		}

		ext := filepath.Ext(path)
		validExt := false
		for _, e := range exts {
			if ext == e {
				validExt = true
				break
			}
		}
		if !validExt {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		var taskName string
		var taskCron string

		scanner := bufio.NewScanner(f)
		var firstCommentLine string
		inBlockComment := false

		// 特殊处理：针对当前文件名的 Cron 关联正则表达式 (对标青龙 perl 逻辑)
		// 寻找类似 "// 0 0 * * * jd_task.js" 的行
		fileNameEscaped := regexp.QuoteMeta(filepath.Base(path))
		associatedCronRegex := regexp.MustCompile(fmt.Sprintf(`(?i)(?:^|[ \t\*\//])(([0-9\*\/\-,L?#]+[ \t]+){4,5}[0-9\*\/\-,L?#]+)[ \t,"]+.*%s`, fileNameEscaped))

		for i := 0; i < 150 && scanner.Scan(); i++ { // QL 扫描范围较大
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			// 处理块注释开始/结束
			if strings.HasPrefix(line, "/*") {
				inBlockComment = true
				line = strings.TrimPrefix(line, "/*")
				line = strings.TrimPrefix(line, "*")
				line = strings.TrimSpace(line)
			}
			if strings.HasSuffix(line, "*/") {
				inBlockComment = false
				line = strings.TrimSuffix(line, "*/")
				line = strings.TrimSpace(line)
			}

			// 1. 尝试提取任务名称 (优先使用 Env)
			if taskName == "" {
				if envMatch := envRegex.FindStringSubmatch(line); len(envMatch) > 1 {
					taskName = strings.TrimSpace(envMatch[1])
				} else if strings.Contains(line, "name:") {
					// 兼容 name: "xxx" 格式
					nameRegex := regexp.MustCompile(`(?i)name:[ \t]*['"]([^'"]+)['"]`)
					if nameMatch := nameRegex.FindStringSubmatch(line); len(nameMatch) > 1 {
						taskName = strings.TrimSpace(nameMatch[1])
					}
				}
			}

			// 如果还没找到名称，且在注释中，记录第一行非空注释作为备选名称
			if taskName == "" && (inBlockComment || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "#")) {
				cleanLine := line
				if strings.HasPrefix(line, "//") {
					cleanLine = strings.TrimPrefix(line, "//")
				} else if strings.HasPrefix(line, "#") {
					cleanLine = strings.TrimPrefix(line, "#")
				} else if strings.HasPrefix(line, "*") {
					cleanLine = strings.TrimPrefix(line, "*")
				}
				cleanLine = strings.TrimSpace(cleanLine)
				
				// 排除掉包含 "Env" 或 "cron" 的行, 且排除掉可能是路径或URL的行
				if cleanLine != "" && !strings.Contains(strings.ToLower(cleanLine), "env") && 
				   !strings.Contains(strings.ToLower(cleanLine), "cron") && 
				   !strings.Contains(cleanLine, "http") &&
				   !strings.Contains(cleanLine, "/") &&
				   firstCommentLine == "" {
					// 且排除掉纯 cron 表达式
					if !cronRegex.MatchString(cleanLine) {
						firstCommentLine = cleanLine
					}
				}
			}

			// 2. 提取 Cron
			if taskCron == "" {
				// A. 优先查找关联了当前文件名的 Cron (对标 QL)
				if assocMatch := associatedCronRegex.FindStringSubmatch(line); len(assocMatch) > 1 {
					taskCron = strings.TrimSpace(assocMatch[1])
				}
				
				// B. 如果没找到，尝试普通的 cron: "..." 或 cron 表达式
				if taskCron == "" {
					if cronMatch := cronRegex.FindStringSubmatch(line); len(cronMatch) > 0 {
						for _, m := range cronMatch[1:] {
							if m != "" {
								taskCron = strings.TrimSpace(m)
								break
							}
						}
					}
				}
			}

			if taskName != "" && taskCron != "" {
				break
			}
		}

		// 如果最后还是没找到 taskName，使用备选名称或文件名
		if taskName == "" {
			if firstCommentLine != "" {
				taskName = firstCommentLine
			} else {
				taskName = strings.TrimSuffix(filepath.Base(path), ext)
			}
		}

		// 3. 应用白名单 / 黑名单过滤 (逻辑对标青龙)
		// relRepoPath 是相对于仓库根目录的路径，filename 是文件名
		relRepoPath, _ := filepath.Rel(targetPath, path)
		filename := filepath.Base(path)

		// 只有在显式设置了白名单时才进行白名单校验 (青龙行为)
		if repoCfg.WhitelistPaths != "" {
			if !matchesQLPattern(relRepoPath, filename, repoCfg.WhitelistPaths) {
				return nil
			}
		}

		// 校验黑名单
		if repoCfg.Blacklist != "" {
			if matchesQLPattern(relRepoPath, filename, repoCfg.Blacklist) {
				return nil
			}
		}

		if taskName != "" && taskCron != "" {
			// 获取脚本相对于数据目录的路径
			absScriptsDir := resolveAbsScriptsDir()
			absTargetPath, _ := filepath.Abs(targetPath)
			absPath, _ := filepath.Abs(path)
			
			// 计算 SourceID: 相对于脚本目录的完整路径，并清洗特殊符号
			relPath, _ := filepath.Rel(absScriptsDir, absPath)
			sourceID := sanitizeIdentifier(relPath)

			// 替换绝对路径为代号 $SCRIPTS_DIR$
			displayPath := path
			displayWorkDir := targetPath
			if strings.HasPrefix(absPath, absScriptsDir) {
				if relCommandPath, err := filepath.Rel(absTargetPath, absPath); err == nil && relCommandPath != "" {
					displayPath = filepath.Clean(relCommandPath)
				}
                // 获取目录路径
                relDir, _ := filepath.Rel(absScriptsDir, absTargetPath)
                displayWorkDir = filepath.Join("$SCRIPTS_DIR$", relDir)
			}

			// Found task, save it
			command := getCommandByExt(ext, displayPath)

			// 注册任务默认开启“全量环境变量注入”，以适配大多数脚本
			defaultTaskConfig := `{"$task_all_envs":true}`

			// See if task exists (优先通过 SourceID 匹配)
			var existing models.Task
			tx := database.DB.Where("source_id = ? AND repo_task_id = ?", sourceID, repoTask.ID).Limit(1).Find(&existing)

			if tx.RowsAffected > 0 {
				// update
				existing.Name = taskName
				existing.Command = models.BigText(command)
				existing.Schedule = normalizeCron(taskCron)
				existing.Languages = repoTask.Languages
				existing.SourceID = sourceID
				existing.RepoTaskID = repoTask.ID
				existing.WorkDir = displayWorkDir
				// 如果原配置为空或者是 {}，则应用默认配置
				if string(existing.Config) == "" || string(existing.Config) == "{}" {
					existing.Config = models.BigText(defaultTaskConfig)
				}
				// 默认开启按条数清理30条
				if existing.CleanConfig == "" {
					existing.CleanConfig = `{"type":"count","keep":30}`
				}
				database.DB.Save(&existing)
				
				if existing.Enabled && es != nil {
					es.AddCronTask(&existing)
				}
				log("[更新] 任务: %s (%s)", taskName, filename)
				updateTaskCount++
				foundSourceIDs[sourceID] = true
			} else {
				// create new
				newTask := &models.Task{
					Name:        taskName,
					Command:     models.BigText(command),
					Schedule:    normalizeCron(taskCron),
					Type:        "task",
					TriggerType: constant.TriggerTypeCron,
					Tags:        tag,
					Languages:   repoTask.Languages,
					Timeout:     repoTask.Timeout,
					Config:      models.BigText(defaultTaskConfig),
					Enabled:     true,
					WorkDir:     displayWorkDir,
					SourceID:    sourceID,
					RepoTaskID:  repoTask.ID,
					CleanConfig: `{"type":"count","keep":30}`,
				}
				newTask.ID = utils.GenerateID()
				database.DB.Create(newTask)
				if es != nil {
					es.AddCronTask(newTask)
				}
				log("[新增] 任务: %s (%s)", taskName, filename)
				newTaskCount++
				foundSourceIDs[sourceID] = true
			}
		}

		return nil
	})

	// 清理该仓库任务下不再存在的旧脚本任务
	deletedTaskCount := 0
	var oldTasks []models.Task
	if err := database.DB.Where("repo_task_id = ?", repoTask.ID).Find(&oldTasks).Error; err == nil {
		for _, ot := range oldTasks {
			if !foundSourceIDs[ot.SourceID] {
				log("[移除] 脚本已不存在，删除对应任务: %s", ot.Name)
				deletedTaskCount++
				if es != nil {
					if es.taskService != nil {
						es.taskService.DeleteTask(ot.ID)
					}
					es.RemoveCronTask(ot.ID)
				} else {
					// Fallback if es is nil (which shouldn't happen, but just in case)
					database.DB.Unscoped().Where("id = ?", ot.ID).Delete(&models.Task{})
				}
			}
		}
	}

	log("\n扫描完成: [新增 %d] [更新 %d] [移除 %d]", newTaskCount, updateTaskCount, deletedTaskCount)
	log("----------------------------------------")
}

func sanitizeIdentifier(s string) string {
	// 将所有非字母数字替换为下划线
	reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	res := reg.ReplaceAllString(s, "_")
	return strings.ToLower(strings.Trim(res, "_"))
}

func normalizeCron(cron string) string {
	fields := strings.Fields(cron)
	if len(fields) == 5 {
		return "0 " + cron
	}
	return cron
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func getCommandByExt(ext, path string) string {
	quotedPath := utils.QuotePath(path)
	switch ext {
	case ".js", ".ts":
		return fmt.Sprintf("node %s", quotedPath)
	case ".py":
		return fmt.Sprintf("python %s", quotedPath)
	case ".sh":
		return fmt.Sprintf("bash %s", quotedPath)
	case ".php":
		return fmt.Sprintf("php %s", quotedPath)
	}
	return quotedPath
}

func matchesQLPattern(rel, filename string, keywordsStr string) bool {
	if keywordsStr == "" {
		return false
	}
	
	keywords := splitKeywords(keywordsStr)
	for _, k := range keywords {
		// 1. 尝试作为正则整体进行匹配，默认不区分大小写 (?i)
		pattern := k
		if !strings.HasPrefix(pattern, "(?i)") {
			pattern = "(?i)" + pattern
		}

		reg, err := regexp.Compile(pattern)
		if err == nil {
			// 优先匹配文件名（解决 ^jd[^_] 这种锚点在相对路径下失效的问题）
			if reg.MatchString(filename) || reg.MatchString(rel) {
				return true
			}
		} else {
			// 回退逻辑：全小写包含判断
			kLower := strings.ToLower(k)
			if strings.Contains(strings.ToLower(rel), kLower) || strings.Contains(strings.ToLower(filename), kLower) {
				return true
			}
		}
	}
	return false
}

func splitKeywords(s string) []string {
	if s == "" {
		return nil
	}
	var parts []string
	if strings.Contains(s, "|") {
		parts = strings.Split(s, "|")
	} else if strings.Contains(s, ",") {
		parts = strings.Split(s, ",")
	} else {
		parts = []string{s}
	}
	
	var res []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			res = append(res, p)
		}
	}
	return res
}
