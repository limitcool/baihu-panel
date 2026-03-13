package controllers

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/services/manifest"
	"github.com/engigu/baihu-panel/internal/utils"
	"github.com/gin-gonic/gin"
)

type ManifestController struct {
	service  *manifest.ManifestService
	workDir  string
}

func NewManifestController(workDir string) *ManifestController {
	return &ManifestController{
		service: manifest.NewManifestService(),
		workDir: workDir,
	}
}

func (mc *ManifestController) Export(c *gin.Context) {
	var req struct {
		Paths []string             `json:"paths"`
		Tools map[string]string    `json:"tools"`
		Envs  []models.BaihuEnv    `json:"envs"`
		Tasks []models.BaihuTask   `json:"tasks"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	// Build manifest
	m := &models.BaihuManifest{
		Tools: req.Tools,
		Baihu: struct {
			Envs  []models.BaihuEnv  `yaml:"envs,omitempty" json:"envs,omitempty"`
			Tasks []models.BaihuTask `yaml:"tasks,omitempty" json:"tasks,omitempty"`
		}{
			Envs:  req.Envs,
			Tasks: req.Tasks,
		},
	}

	// Create temporary zip file
	tmpDir := os.TempDir()
	zipName := fmt.Sprintf("export_%d.zip", time.Now().UnixNano())
	zipPath := filepath.Join(tmpDir, zipName)

	if err := mc.service.Export(zipPath, mc.workDir, m, req.Paths); err != nil {
		utils.ServerError(c, "导出失败: "+err.Error())
		return
	}
	defer os.Remove(zipPath)

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename=project_export.zip")
	c.Header("Content-Type", "application/octet-stream")
	c.File(zipPath)
}
