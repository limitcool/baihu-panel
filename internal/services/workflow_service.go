package services

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/database"
	"github.com/engigu/baihu-panel/internal/models"
	"gorm.io/gorm"
)

type WorkflowService struct{}

func NewWorkflowService() *WorkflowService {
	return &WorkflowService{}
}

// List 获取工作流列表
func (s *WorkflowService) List(page, pageSize int, name string) ([]models.Workflow, int64, error) {
	var workflows []models.Workflow
	var total int64
	query := database.DB.Table(models.Workflow{}.TableName())

	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}

	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&workflows).Error
	return workflows, total, err
}

func (s *WorkflowService) GetByID(id string) (*models.Workflow, error) {
	var workflow models.Workflow
	// 联表查询：加载节点、连线，以及节点对应的白虎脚本（为了获取最新名称）
	// 使用 Model(&models.Workflow{}) 确保 Preload 能正确解析关联关系
	err := database.DB.Model(&models.Workflow{}).
		Preload("Nodes.Task").
		Preload("Edges").
		Where("id = ?", id).
		First(&workflow).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("workflow not found")
		}
		return nil, err
	}

	// 批量根据 taskId 去匹配最新的脚本名称：
	// 如果关联了 Task，则更新 Node.Label 为最新的脚本名称
	for i := range workflow.Nodes {
		node := &workflow.Nodes[i]
		if node.TaskID != "" && node.TaskID != constant.WorkflowVirtualTaskID && node.Task != nil {
			node.Label = node.Task.Name
		}
	}

	return &workflow, nil
}

// Create 创建工作流
func (s *WorkflowService) Create(workflow *models.Workflow) error {
	return database.DB.Create(workflow).Error
}

// Update 更新工作流，实现数据拆分与版本快照
func (s *WorkflowService) Update(workflow *models.Workflow, rawFlowData string) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		// 1. 更新主表基础信息
		updates := map[string]interface{}{
			"name":        workflow.Name,
			"description": workflow.Description,
			"schedule":    workflow.Schedule,
			"enabled":     workflow.Enabled,
			"next_run":    workflow.NextRun,
		}
		if err := tx.Table(models.Workflow{}.TableName()).Where("id = ?", workflow.ID).Updates(updates).Error; err != nil {
			return err
		}

		// 如果没有传入新的流程数据，则只更新基础信息
		if rawFlowData == "" {
			return nil
		}

		// 2. 创建版本快照
		version := &models.WorkflowVersion{
			WorkflowID: workflow.ID,
			FlowData:   rawFlowData,
		}
		if err := tx.Create(version).Error; err != nil {
			return err
		}

		// 3. 拆分解析流程数据并存入 Nodes/Edges 表
		var flow struct {
			Nodes []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Position struct {
					X float64 `json:"x"`
					Y float64 `json:"y"`
				} `json:"position"`
				Label string `json:"label"`
				Data  struct {
					TaskID      string `json:"taskId"`
					NodeType    string `json:"nodeType"`
					ControlType string `json:"controlType"`
					Config      string `json:"config"`
				} `json:"data"`
			} `json:"nodes"`
			Edges []struct {
				ID           string `json:"id"`
				Source       string `json:"source"`
				Target       string `json:"target"`
				Label        string `json:"label"`
				SourceHandle string `json:"sourceHandle"`
				Data         struct {
					Condition string `json:"condition"`
				} `json:"data"`
			} `json:"edges"`
		}

		if err := json.Unmarshal([]byte(rawFlowData), &flow); err != nil {
			return fmt.Errorf("解析流程数据失败: %v", err)
		}

		// 清理旧节点与连线 (实现全量覆盖)
		tx.Table(models.WorkflowNode{}.TableName()).Where("workflow_id = ?", workflow.ID).Delete(&models.WorkflowNode{})
		tx.Table(models.WorkflowEdge{}.TableName()).Where("workflow_id = ?", workflow.ID).Delete(&models.WorkflowEdge{})

		// 批量插入新节点
		for _, n := range flow.Nodes {
			node := &models.WorkflowNode{
				ID:          n.ID,
				WorkflowID:  workflow.ID,
				TaskID:      n.Data.TaskID,
				NodeType:    n.Data.NodeType,
				ControlType: n.Data.ControlType,
				Config:      n.Data.Config,
				Label:       n.Label,
				Type:        n.Type,
				PosX:        n.Position.X,
				PosY:        n.Position.Y,
			}
			if err := tx.Create(node).Error; err != nil {
				return err
			}
		}

		// 批量插入新连线
		for _, e := range flow.Edges {
			edge := &models.WorkflowEdge{
				ID:           e.ID,
				WorkflowID:   workflow.ID,
				Source:       e.Source,
				Target:       e.Target,
				Label:        e.Label,
				Condition:    e.Data.Condition,
				SourceHandle: e.SourceHandle,
			}
			if err := tx.Create(edge).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// Delete 删除工作流（同时清理关联数据）
func (s *WorkflowService) Delete(id string) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		tx.Table(models.WorkflowNode{}.TableName()).Where("workflow_id = ?", id).Delete(nil)
		tx.Table(models.WorkflowEdge{}.TableName()).Where("workflow_id = ?", id).Delete(nil)
		tx.Table(models.WorkflowVersion{}.TableName()).Where("workflow_id = ?", id).Delete(nil)
		return tx.Table(models.Workflow{}.TableName()).Where("id = ?", id).Delete(nil).Error
	})
}

// ToggleStatus 切换工作流状态
func (s *WorkflowService) ToggleStatus(id string, enabled bool) error {
	return database.DB.Table(models.Workflow{}.TableName()).Where("id = ?", id).Update("enabled", enabled).Error
}
