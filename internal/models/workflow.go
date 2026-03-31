package models

import (
	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/rs/xid"
	"gorm.io/gorm"
)

// Workflow 代表一个可视化脚本编排工作流
type Workflow struct {
	ID          string         `json:"id" gorm:"size:20;primaryKey"`
	Name        string         `json:"name" gorm:"size:255;not null"`
	Description string         `json:"description" gorm:"size:1024;default:''"`
	Schedule    string         `json:"schedule" gorm:"size:100"`    // 整体重跑的 Cron 表达式
	Enabled     bool           `json:"enabled" gorm:"default:true"` // 总开关
	LastRun     *LocalTime     `json:"last_run"`
	NextRun     *LocalTime     `json:"next_run"`
	CreatedAt   LocalTime      `json:"created_at"`
	UpdatedAt   LocalTime      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`

	// 关联当前激活状态的节点与连线
	Nodes []WorkflowNode `json:"nodes,omitempty" gorm:"foreignKey:WorkflowID"`
	Edges []WorkflowEdge `json:"edges,omitempty" gorm:"foreignKey:WorkflowID"`
}

func (Workflow) TableName() string {
	return constant.TablePrefix + "workflows"
}

func (w *Workflow) BeforeCreate(tx *gorm.DB) (err error) {
	if w.ID == "" {
		w.ID = xid.New().String()
	}
	return
}

// WorkflowVersion 存储工作流的历史版本快照
type WorkflowVersion struct {
	ID         string    `json:"id" gorm:"size:20;primaryKey"`
	WorkflowID string    `json:"workflow_id" gorm:"size:20;index"`
	FlowData   string    `json:"flow_data" gorm:"type:longtext"` // 保存当时的完整 JSON 快照
	CreatedAt  LocalTime `json:"created_at"`
}

func (WorkflowVersion) TableName() string {
	return constant.TablePrefix + "workflow_versions"
}

func (wv *WorkflowVersion) BeforeCreate(tx *gorm.DB) (err error) {
	if wv.ID == "" {
		wv.ID = xid.New().String()
	}
	return
}

// WorkflowNode 结构化存储当前工作流中的每一个节点
type WorkflowNode struct {
	ID          string `json:"id" gorm:"primaryKey"` // VueFlow 内部 ID (如 node-123)
	WorkflowID  string `json:"workflow_id" gorm:"size:20;primaryKey"`
	TaskID      string `json:"taskId" gorm:"size:20"` // 对应的白虎脚本 ID
	NodeType    string `json:"nodeType" gorm:"size:50"`
	ControlType string `json:"controlType" gorm:"size:50"`
	Config      string `json:"config" gorm:"type:text"` // 节点私有配置（如延时时间、条件表达式）
	Label       string `json:"label" gorm:"size:255"`   // 缓存的显示文本
	Type        string `json:"type" gorm:"size:50"`    // VueFlow 视觉类型 (input/output/default)
	PosX        float64 `json:"posX"`
	PosY        float64 `json:"posY"`

	// 动态关联：用于返回给前端时匹配最新的脚本名称
	Task *Task `json:"task,omitempty" gorm:"foreignKey:TaskID;references:ID"`
}

func (WorkflowNode) TableName() string {
	return constant.TablePrefix + "workflow_nodes"
}

// WorkflowEdge 结构化存储工作流中的连线与分支关系
type WorkflowEdge struct {
	ID           string `json:"id" gorm:"primaryKey"` // VueFlow 内部 ID (如 edge-123)
	WorkflowID   string `json:"workflow_id" gorm:"size:20;primaryKey"`
	Source       string `json:"source" gorm:"size:100;index"`
	Target       string `json:"target" gorm:"size:100;index"`
	Label        string `json:"label" gorm:"size:100"`
	Condition    string `json:"condition" gorm:"size:50"` // 触发连线的条件 (always/success/failed)
	SourceHandle string `json:"sourceHandle" gorm:"size:50"`
}

func (WorkflowEdge) TableName() string {
	return constant.TablePrefix + "workflow_edges"
}

// WorkflowRun 存储工作流的一次完整运行记录状态
type WorkflowRun struct {
	ID         string    `json:"id" gorm:"size:20;primaryKey"`
	WorkflowID string    `json:"workflow_id" gorm:"size:20;index"`
	Status     string    `json:"status" gorm:"size:20"` // running, success, failed
	StartTime  LocalTime `json:"start_time"`
	EndTime    *LocalTime `json:"end_time"`
}

func (WorkflowRun) TableName() string {
	return constant.TablePrefix + "workflow_runs"
}

func (wr *WorkflowRun) BeforeCreate(tx *gorm.DB) (err error) {
	if wr.ID == "" {
		wr.ID = xid.New().String()
	}
	return
}
