package tasks

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/engigu/baihu-panel/internal/constant"
	"github.com/engigu/baihu-panel/internal/database"
	"github.com/engigu/baihu-panel/internal/logger"
	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/utils"
)

var outputRegex = regexp.MustCompile(`(?i)output\.([a-zA-Z0-9_-]+)\s*=\s*(.*)`)

// TriggerWorkflowNextTasks 当一个脚本结束时，遍历所有开启的工作流并检查是否有满足触发条件的分支，自动触发下一级
func (es *ExecutorService) TriggerWorkflowNextTasks(taskLog *models.TaskLog, extraEnvs []string) {
	// 如果脚本非完成状态（成功或失败），则忽略
	if taskLog.Status != constant.TaskStatusSuccess && taskLog.Status != constant.TaskStatusFailed {
		return
	}

	// 查出所有启用状态的工作流，并预加载它们的节点和连线
	var workflows []models.Workflow
	if err := database.DB.Table(models.Workflow{}.TableName()).
		Preload("Nodes").
		Preload("Edges").
		Where("enabled = ?", true).
		Find(&workflows).Error; err != nil {
		logger.Errorf("[Workflow] 查找可用工作流失败: %v", err)
		return
	}

	// 提取脚本输出中的变量：扫描日志获取 output.name=val 格式
	// 存储在 map 中，确保最后一次出现的值覆盖之前的。
	outputs := make(map[string]string)
	if taskLog.Output != "" {
		rawOutput, _ := utils.DecompressFromBase64(string(taskLog.Output))
		lines := strings.Split(rawOutput, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			matches := outputRegex.FindStringSubmatch(line)
			if len(matches) == 3 {
				key := strings.TrimSpace(matches[1])
				val := strings.TrimSpace(matches[2])
				outputs[key] = val
			}
		}
	}

	// 将提取到的输出变量转换为下游的输入变量 (input.name=val)
	capturedEnvs := make([]string, 0)
	// 继承传下来的 context (清理掉旧的 input.xxx 防止冲突可以考虑，但通常追加即可)
	for _, env := range extraEnvs {
		if !strings.HasPrefix(env, "input.") {
			capturedEnvs = append(capturedEnvs, env)
		}
	}
	for k, v := range outputs {
		capturedEnvs = append(capturedEnvs, fmt.Sprintf("input.%s=%s", k, v))
	}

	for _, wf := range workflows {
		// 建立 NodeID 到 配置的映射（直接从结构化表中读取）
		nodeIdToTaskId := make(map[string]string)
		nodeIdToType := make(map[string]string)
		nodeIdToControlType := make(map[string]string)
		nodeIdToConfig := make(map[string]string)
		
		for _, n := range wf.Nodes {
			nodeIdToTaskId[n.ID] = n.TaskID
			nodeIdToType[n.ID] = n.NodeType
			nodeIdToControlType[n.ID] = n.ControlType
			nodeIdToConfig[n.ID] = n.Config
		}

		// 遍历工作流连线寻找目标
		for _, edge := range wf.Edges {
			sourceTaskId := nodeIdToTaskId[edge.Source]
			if sourceTaskId != taskLog.TaskID {
				continue
			}

			// 匹配条件
			condition := edge.SourceHandle
			if condition == "" {
				condition = edge.Condition
			}

			match := false
			if (condition == constant.WorkflowConditionSuccess || condition == constant.WorkflowConditionOnSuccess) && taskLog.Status == constant.TaskStatusSuccess {
				match = true
			} else if (condition == constant.WorkflowConditionError || condition == constant.WorkflowConditionFailed || condition == constant.WorkflowConditionOnError) && taskLog.Status == constant.TaskStatusFailed {
				match = true
			} else if condition == "" || condition == constant.WorkflowConditionAlways {
				match = true
			}

			if match {
				targetNodeId := edge.Target
				targetTaskId := nodeIdToTaskId[targetNodeId]
				
				if targetTaskId != "" {
					logger.Infof("[Workflow] 脚本 #%s 执行%s，触发流程 (WF: #%s) 节点 %s", taskLog.TaskID, taskLog.Status, wf.ID, targetNodeId)
					
					runID := taskLog.WorkflowRunID
					if runID == "" {
						runID = fmt.Sprintf("WF-RUN-%d", time.Now().UnixMilli())
					}
					
					go func(tid string, wfid string, rid string, extraEnvs []string, nType string, nCtrlType string, nConfig string) {
						if tid == constant.WorkflowVirtualTaskID {
							es.triggerControlNode(tid, nCtrlType, nConfig, wfid, rid, extraEnvs)
							return
						}

						time.Sleep(time.Millisecond * 500) 
						envs := []string{
							fmt.Sprintf("BAIHU_WF_ID=%s", wfid),
							fmt.Sprintf("BAIHU_WF_RUN_ID=%s", rid),
						}
						envs = append(envs, extraEnvs...)
						es.ExecuteTask(tid, envs)
					}(targetTaskId, wf.ID, runID, capturedEnvs, nodeIdToType[targetNodeId], nodeIdToControlType[targetNodeId], nodeIdToConfig[targetNodeId])
				}
			}
		}
	}
}

// triggerControlNode 处理虚拟控制节点（如：延时、分支判断、循环）
func (es *ExecutorService) triggerControlNode(targetNodeTaskId string, controlType string, config string, wfID string, wfRunID string, envs []string) {
	nodeName := controlType
	logService := &TaskLogService{}
	taskLog, _ := logService.CreateEmptyLog(targetNodeTaskId, "Workflow Control: "+nodeName, &wfID, wfRunID)

	switch controlType {
	case constant.WorkflowControlDelay:
		delaySec := 5 
		fmt.Sscanf(config, "%d", &delaySec)
		logger.Infof("[Workflow] 控制节点 (延时) 等待 %d 秒 (Run: %s)", delaySec, wfRunID)
		
		go func() {
			time.Sleep(time.Duration(delaySec) * time.Second)
			taskLog.Status = constant.TaskStatusSuccess
			now := models.Now()
			taskLog.EndTime = &now
			out, _ := utils.CompressToBase64(fmt.Sprintf("Wait completed after %d seconds.", delaySec))
			taskLog.Output = models.BigText(out)
			logService.ProcessTaskCompletion(taskLog)
			es.TriggerWorkflowNextTasks(taskLog, envs)
		}()

	case constant.WorkflowControlCondition:
		logger.Infof("[Workflow] 控制节点 (条件判断) 执行: %s (Run: %s)", config, wfRunID)
		result := es.evaluateConditionExpression(config, envs)
		
		taskLog.Status = constant.TaskStatusSuccess
		if !result {
			taskLog.Status = constant.TaskStatusFailed 
		}
		
		now := models.Now()
		taskLog.EndTime = &now
		out, _ := utils.CompressToBase64(fmt.Sprintf("Condition: %s, Result: %v", config, result))
		taskLog.Output = models.BigText(out)
		logService.ProcessTaskCompletion(taskLog)
		es.TriggerWorkflowNextTasks(taskLog, envs)

	case constant.WorkflowControlLoop:
		maxLoops := 1
		fmt.Sscanf(config, "%d", &maxLoops)
		
		var count int64
		database.DB.Model(&models.TaskLog{}).
			Where("workflow_run_id = ? AND task_id = ? AND status = ?", wfRunID, targetNodeTaskId, constant.TaskStatusSuccess).
			Count(&count)
		
		logger.Infof("[Workflow] 控制节点 (循环) 进度: %d/%d (Run: %s)", count+1, maxLoops, wfRunID)
		
		if int(count) < maxLoops {
			taskLog.Status = constant.TaskStatusSuccess
		} else {
			taskLog.Status = constant.TaskStatusFailed 
		}
		
		now := models.Now()
		taskLog.EndTime = &now
		out, _ := utils.CompressToBase64(fmt.Sprintf("Loop Count: %d/%d", count+1, maxLoops))
		taskLog.Output = models.BigText(out)
		logService.ProcessTaskCompletion(taskLog)
		es.TriggerWorkflowNextTasks(taskLog, envs)

	default:
		taskLog.Status = constant.TaskStatusSuccess
		now := models.Now()
		taskLog.EndTime = &now
		logService.ProcessTaskCompletion(taskLog)
		es.TriggerWorkflowNextTasks(taskLog, envs)
	}
}

// evaluateConditionExpression 评估表达式
func (es *ExecutorService) evaluateConditionExpression(expr string, envs []string) bool {
	if expr == "" {
		return true
	}
	processed := expr
	for _, env := range envs {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			key := parts[0]
			val := parts[1]
			// 支持 {{input.name}} 或 {{name}} 格式替换
			processed = strings.ReplaceAll(processed, "{{"+key+"}}", val)
			if strings.HasPrefix(key, "input.") {
				shortKey := strings.TrimPrefix(key, "input.")
				processed = strings.ReplaceAll(processed, "{{"+shortKey+"}}", val)
			}
		}
	}
	if strings.Contains(processed, "==") {
		sides := strings.Split(processed, "==")
		if len(sides) == 2 {
			return strings.TrimSpace(sides[0]) == strings.TrimSpace(sides[1])
		}
	}
	return strings.TrimSpace(processed) != "" && !strings.Contains(processed, "{")
}

// TriggerWorkflow 手动或定时执行工作流
func (es *ExecutorService) TriggerWorkflow(workflowID string, extraEnvs []string) error {
	var wf models.Workflow
	err := database.DB.Table(models.Workflow{}.TableName()).
		Preload("Nodes").
		Preload("Edges").
		Where("id = ?", workflowID).
		First(&wf).Error
	if err != nil {
		return fmt.Errorf("工作流未找到: %v", err)
	}

	if !wf.Enabled {
		return fmt.Errorf("工作流已被禁用")
	}

	hasIncomingEdges := make(map[string]bool)
	for _, edge := range wf.Edges {
		hasIncomingEdges[edge.Target] = true
	}

	runID := fmt.Sprintf("WF-RUN-%d", time.Now().UnixMilli())
	now := models.LocalTime(time.Now())
	database.DB.Model(&wf).Update("last_run", &now)

	for _, node := range wf.Nodes {
		if hasIncomingEdges[node.ID] {
			continue
		}
		
		tid := node.TaskID
		if tid == "" {
			continue
		}

		go func(taskID string, nType string, nCtrlType string, nConfig string) {
			if taskID == constant.WorkflowVirtualTaskID {
				es.triggerControlNode(taskID, nCtrlType, nConfig, wf.ID, runID, extraEnvs)
				return
			}

			logger.Infof("[Workflow] 触发启动工作流 (WF: #%s), 启动根节点 %s", wf.ID, taskID)
			envs := []string{
				fmt.Sprintf("BAIHU_WF_ID=%s", wf.ID),
				fmt.Sprintf("BAIHU_WF_RUN_ID=%s", runID),
			}
			envs = append(envs, extraEnvs...)
			es.ExecuteTask(taskID, envs)
		}(tid, node.NodeType, node.ControlType, node.Config)
	}

	return nil
}
