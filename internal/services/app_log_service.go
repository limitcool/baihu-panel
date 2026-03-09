package services

import (
	"encoding/json"
	"time"

	"fmt"

	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/database"
	"github.com/engigu/baihu-panel/internal/eventbus"
	"github.com/engigu/baihu-panel/internal/logger"
	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/utils"
)

type LogRetentionConfig struct {
	Days     int `json:"days"`
	MaxCount int `json:"max_count"`
}

type AppLogService struct {
	settingsService *SettingsService
}

func NewAppLogService() *AppLogService {
	return &AppLogService{
		settingsService: NewSettingsService(),
	}
}

func (s *AppLogService) Add(log *models.AppLog) error {
	if log.ID == "" {
		log.ID = utils.GenerateID()
	}
	return database.DB.Create(log).Error
}

func (s *AppLogService) List(category string, status string, level string, page, pageSize int, keyword string) ([]models.AppLog, int64, error) {
	var logs []models.AppLog
	var total int64
	query := database.DB.Model(&models.AppLog{})

	if category != "" {
		query = query.Where("category = ?", category)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if level != "" {
		query = query.Where("level = ?", level)
	}
	if keyword != "" {
		query = query.Where("(title LIKE ? OR content LIKE ?)", "%"+keyword+"%", "%"+keyword+"%")
	}

	query.Count(&total)
	offset := (page - 1) * pageSize
	err := query.Order("created_at desc").Offset(offset).Limit(pageSize).Find(&logs).Error
	return logs, total, err
}

func (s *AppLogService) MarkAsRead(id string) error {
	now := models.LocalTime(time.Now())
	return database.DB.Model(&models.AppLog{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":  constant.LogStatusRead,
		"read_at": &now,
	}).Error
}

func (s *AppLogService) MarkAllAsRead(category string) error {
	now := models.LocalTime(time.Now())
	return database.DB.Model(&models.AppLog{}).Where("category = ? AND status = ?", category, constant.LogStatusUnread).Updates(map[string]interface{}{
		"status":  constant.LogStatusRead,
		"read_at": &now,
	}).Error
}

func (s *AppLogService) Clear(category string) error {
	query := database.DB.Model(&models.AppLog{})
	if category != "" {
		query = query.Where("category = ?", category)
	}
	return query.Unscoped().Delete(&models.AppLog{}).Error
}

func (s *AppLogService) GetRetentionConfigs() map[string]LogRetentionConfig {
	val := s.settingsService.Get(constant.SectionSystem, "log_retention")
	var configs map[string]LogRetentionConfig
	if val != "" {
		_ = json.Unmarshal([]byte(val), &configs)
	}
	if configs == nil {
		configs = map[string]LogRetentionConfig{
			constant.LogCategorySystemNotice: {Days: 30, MaxCount: 500},
			constant.LogCategoryPushLog:      {Days: 15, MaxCount: 5000},
			constant.LogCategoryDefault:      {Days: 30, MaxCount: 10000},
		}
	}
	return configs
}

func (s *AppLogService) CleanUp() {
	configs := s.GetRetentionConfigs()
	categories := []string{constant.LogCategorySystemNotice, constant.LogCategoryPushLog}

	for _, cat := range categories {
		cfg, ok := configs[cat]
		if !ok {
			cfg = configs[constant.LogCategoryDefault]
		}

		if cfg.Days > 0 {
			deadline := time.Now().AddDate(0, 0, -cfg.Days)
			database.DB.Unscoped().Where("category = ? AND created_at < ?", cat, deadline).Delete(&models.AppLog{})
		}

		if cfg.MaxCount > 0 {
			var total int64
			database.DB.Model(&models.AppLog{}).Where("category = ?", cat).Count(&total)
			if total > int64(cfg.MaxCount) {
				deleteCount := total - int64(cfg.MaxCount)
				var ids []string
				database.DB.Model(&models.AppLog{}).Where("category = ?", cat).Order("created_at asc").Limit(int(deleteCount)).Pluck("id", &ids)
				if len(ids) > 0 {
					database.DB.Unscoped().Where("id IN ?", ids).Delete(&models.AppLog{})
				}
			}
		}
	}
	logger.Debugf("[AppLog] 完成应用日志清理策略")
}

func (s *AppLogService) SubscribeEvents(bus *eventbus.EventBus) {
	// 1. [订阅] 系统通知 -> 存储到数据库表现为红点消息
	bus.Subscribe(constant.EventSystemNotice, func(e eventbus.Event) {
		payload, ok := e.Payload.(map[string]interface{})
		if !ok {
			return
		}

		title, _ := payload["title"].(string)
		content, _ := payload["content"].(string)
		level, _ := payload["level"].(string)
		if level == "" {
			level = constant.LogLevelInfo
		}

		s.Add(&models.AppLog{
			Category: constant.LogCategorySystemNotice,
			Title:    title,
			Content:  models.BigText(content),
			Level:    level,
			Status:   constant.LogStatusUnread,
		})
	})

	// 2. [订阅] 推送结果 -> 存储到数据库供推送日志查看
	bus.Subscribe(constant.EventNotifySent, func(e eventbus.Event) {
		payload, ok := e.Payload.(map[string]interface{})
		if !ok {
			return
		}

		title, _ := payload["title"].(string)
		content, _ := payload["content"].(string)
		success, _ := payload["success"].(bool)
		errorMsg, _ := payload["error_msg"].(string)
		channelName, _ := payload["channel_name"].(string)

		status := constant.LogStatusSuccess
		level := constant.LogLevelInfo
		if !success {
			status = constant.LogStatusFailed
			level = constant.LogLevelError
		}

		s.Add(&models.AppLog{
			Category: constant.LogCategoryPushLog,
			Title:    fmt.Sprintf("[%s] %s", channelName, title),
			Content:  models.BigText(content),
			Level:    level,
			Status:   status,
			ErrorMsg: models.BigText(errorMsg),
		})
	})

	// 3. 将某些业务事件转化为系统内部通知 (自动出现在小铃铛)
	/*
		bus.Subscribe(constant.EventTaskFailed, func(e eventbus.Event) {
			payload, ok := e.Payload.(map[string]interface{})
			if !ok {
				return
			}
			taskName, _ := payload["task_name"].(string)
			errMsg, _ := payload["error"].(string)

			bus.Publish(eventbus.Event{
				Type: constant.EventSystemNotice,
				Payload: map[string]interface{}{
					"title":   fmt.Sprintf("任务 [%s] 执行失败", taskName),
					"content": fmt.Sprintf("错误详情: %s", errMsg),
					"level":   constant.LogLevelError,
				},
			})
		})

		bus.Subscribe(constant.EventTaskTimeout, func(e eventbus.Event) {
			payload, ok := e.Payload.(map[string]interface{})
			if !ok {
				return
			}
			taskName, _ := payload["task_name"].(string)

			bus.Publish(eventbus.Event{
				Type: constant.EventSystemNotice,
				Payload: map[string]interface{}{
					"title":   fmt.Sprintf("任务 [%s] 执行超时", taskName),
					"content": "任务已经超过预设的运行时间并被系统强制中止。",
					"level":   constant.LogLevelWarning,
				},
			})
		})
	*/

	bus.Subscribe(constant.EventPasswordChanged, func(e eventbus.Event) {
		payload, ok := e.Payload.(map[string]interface{})
		if !ok {
			return
		}
		username, _ := payload["username"].(string)

		bus.Publish(eventbus.Event{
			Type: constant.EventSystemNotice,
			Payload: map[string]interface{}{
				"title":   "账户安全提醒",
				"content": fmt.Sprintf("用户 %s 的账号密码已被修改。", username),
				"level":   constant.LogLevelInfo,
			},
		})
	})
}
