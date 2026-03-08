package database

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/engigu/baihu-panel/internal/logger"
	"github.com/engigu/baihu-panel/internal/models"
	"gorm.io/gorm/schema"
)

func Migrate() error {
	// 执行自定义迁移
	if err := customMigrations(); err != nil {
		logger.Warnf("[Database] 自定义迁移警告: %v", err)
	}

	allModels := []interface{}{
		&models.User{},
		&models.Task{},
		&models.TaskLog{},
		&models.Script{},
		&models.EnvironmentVariable{},
		&models.Setting{},
		&models.LoginLog{},
		&models.SendStats{},
		&models.Dependency{},
		&models.Agent{},
		&models.AgentToken{},
		&models.Language{},
		&models.NotifyWay{},
		&models.NotifyBinding{},
	}

	if err := AutoMigrate(allModels...); err != nil {
		return err
	}

	// MySQL 的 TEXT 类型最大 64KB，LONGTEXT 最大 4GB
	// 模型统一使用 type:text 保持跨数据库兼容，这里针对 MySQL 自动升级为 LONGTEXT
	mysqlUpgradeTextColumns(allModels...)

	return nil
}

// mysqlUpgradeTextColumns 反射扫描所有模型，将 gorm tag 中 type:text 的字段在 MySQL 上升级为 LONGTEXT
func mysqlUpgradeTextColumns(allModels ...interface{}) {
	if DB.Dialector.Name() != "mysql" {
		return
	}

	// 获取当前数据库名
	var dbName string
	DB.Raw("SELECT DATABASE()").Scan(&dbName)

	ns := schema.NamingStrategy{}

	for _, model := range allModels {
		typ := reflect.TypeOf(model)
		if typ.Kind() == reflect.Ptr {
			typ = typ.Elem()
		}

		// 获取表名
		tableName := ""
		if tabler, ok := model.(interface{ TableName() string }); ok {
			tableName = tabler.TableName()
		} else {
			continue
		}

		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			gormTag := field.Tag.Get("gorm")
			if gormTag == "" || !hasGormTypeText(gormTag) {
				continue
			}

			// 从 gorm tag 获取列名，没有则用 GORM 命名策略转换
			columnName := parseGormColumn(gormTag)
			if columnName == "" {
				columnName = ns.ColumnName("", field.Name)
			}

			// 检查当前列类型，已经是 longtext 则跳过
			var columnType string
			DB.Raw("SELECT DATA_TYPE FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND COLUMN_NAME = ?",
				dbName, tableName, columnName).Scan(&columnType)
			if strings.EqualFold(columnType, "longtext") {
				continue
			}

			sql := fmt.Sprintf("ALTER TABLE `%s` MODIFY COLUMN `%s` LONGTEXT", tableName, columnName)
			if err := DB.Exec(sql).Error; err != nil {
				logger.Debugf("[Database] MySQL 升级 %s.%s 为 LONGTEXT: %v", tableName, columnName, err)
			}
		}
	}
}

// hasGormTypeText 检查 gorm tag 中是否包含 type:text
func hasGormTypeText(gormTag string) bool {
	for _, part := range strings.Split(gormTag, ";") {
		if kv := strings.SplitN(strings.TrimSpace(part), ":", 2); len(kv) == 2 {
			if strings.TrimSpace(kv[0]) == "type" && strings.EqualFold(strings.TrimSpace(kv[1]), "text") {
				return true
			}
		}
	}
	return false
}

// parseGormColumn 从 gorm tag 中提取 column:xxx
func parseGormColumn(gormTag string) string {
	for _, part := range strings.Split(gormTag, ";") {
		if kv := strings.SplitN(strings.TrimSpace(part), ":", 2); len(kv) == 2 {
			if strings.TrimSpace(kv[0]) == "column" {
				return strings.TrimSpace(kv[1])
			}
		}
	}
	return ""
}

// customMigrations 自定义迁移（处理 AutoMigrate 无法自动完成的变更）
func customMigrations() error {
	// 检查 ql_tokens 表是否存在
	if DB.Migrator().HasTable("ql_tokens") {
		// 将 code 列重命名为 token（如果 code 列存在）
		if DB.Migrator().HasColumn(&models.AgentToken{}, "code") {
			if err := DB.Migrator().RenameColumn(&models.AgentToken{}, "code", "token"); err != nil {
				logger.Debugf("[Database] 重命名 ql_tokens.code 列: %v", err)
			}
		}
	}
	// 移除 deps 表中的 type 字段（如果存在）
	if DB.Migrator().HasColumn(&models.Dependency{}, "type") {
		if err := DB.Migrator().DropColumn(&models.Dependency{}, "type"); err != nil {
			logger.Debugf("[Database] 移除 deps.type 列失败: %v", err)
		} else {
			logger.Infof("[Database] 已成功移除 deps 表中的冗余 type 列")
		}
	}

	return nil
}
