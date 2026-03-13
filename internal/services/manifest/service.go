package manifest

import (
	"fmt"
	"io/ioutil"
	"os"
	// "strings"

	"github.com/engigu/baihu-panel/internal/models"
	"github.com/engigu/baihu-panel/internal/utils"
	"github.com/goccy/go-yaml"
)

type ManifestService struct{}

func NewManifestService() *ManifestService {
	return &ManifestService{}
}

// Parse parses a mise.yml with baihu extensions
func (s *ManifestService) Parse(content []byte) (*models.BaihuManifest, error) {
	var manifest models.BaihuManifest
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse yaml: %w", err)
	}
	return &manifest, nil
}

// ParseFile parses a manifest file from the given path
func (s *ManifestService) ParseFile(path string) (*models.BaihuManifest, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("manifest file not found: %s", path)
	}

	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}

	return s.Parse(content)
}

// ExtractEnvs converts BaihuEnv to models.EnvironmentVariable
func (s *ManifestService) ExtractEnvs(manifest *models.BaihuManifest, userID string) []models.EnvironmentVariable {
	var result []models.EnvironmentVariable
	for _, e := range manifest.Baihu.Envs {
		result = append(result, models.EnvironmentVariable{
			Name:    e.Name,
			Value:   models.BigText(e.Value),
			Remark:  e.Remark,
			Hidden:  e.Hidden,
			Enabled: e.Enabled,
			UserID:  userID,
		})
	}
	return result
}

// ExtractTasks converts BaihuTask to models.Task
func (s *ManifestService) ExtractTasks(manifest *models.BaihuManifest, userID string) []models.Task {
	var result []models.Task
	for _, t := range manifest.Baihu.Tasks {
		task := models.Task{
			Name:          t.Name,
			Command:       models.BigText(t.Command),
			Schedule:      t.Schedule,
			Timeout:       t.Timeout,
			RetryCount:    t.RetryCount,
			RetryInterval: t.RetryInterval,
			RandomRange:   t.RandomRange,
			Enabled:       t.Enabled,
			AgentID:       nil,      // local task
			WorkDir:       t.Path,   // use Path as workdir for distribution
		}

		// Map tools to Languages
		var languages []map[string]string
		if len(t.Tools) > 0 {
			for _, toolName := range t.Tools {
				if version, ok := manifest.Tools[toolName]; ok {
					languages = append(languages, map[string]string{
						"name":    toolName,
						"version": version,
					})
				}
			}
		} else {
			// Use all tools from manifest if no specific tools mentioned for task
			for toolName, version := range manifest.Tools {
				languages = append(languages, map[string]string{
					"name":    toolName,
					"version": version,
				})
			}
		}
		task.Languages = languages
		result = append(result, task)
	}
	return result
}

// ExtractScripts extracts script info from tasks in the manifest
func (s *ManifestService) ExtractScripts(manifest *models.BaihuManifest, userID string) []models.Script {
	var result []models.Script
	for _, t := range manifest.Baihu.Tasks {
		if t.Path != "" {
			result = append(result, models.Script{
				Name:   t.Name,
				UserID: userID,
				// actual file loading logic will be done in deployment service
			})
		}
	}
	return result
}

// GenerateYAML generates a YAML content from BaihuManifest
func (s *ManifestService) GenerateYAML(manifest *models.BaihuManifest) ([]byte, error) {
	return yaml.Marshal(manifest)
}

// Export creates a zip archive containing selected items and the manifest
func (s *ManifestService) Export(destZip string, srcBase string, manifest *models.BaihuManifest, items []string) error {
	yamlContent, err := s.GenerateYAML(manifest)
	if err != nil {
		return fmt.Errorf("failed to generate yaml: %w", err)
	}

	virtualFiles := map[string][]byte{
		"mise.yml": yamlContent,
	}

	return utils.CreateZip(destZip, srcBase, items, virtualFiles)
}

