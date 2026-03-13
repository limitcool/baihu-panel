package manifest

import (
	"fmt"
	"testing"

	"github.com/engigu/baihu-panel/internal/models"
)

func TestManifestParser(t *testing.T) {
	yamlContent := `
tools:
  node: 20.10.0
  python: 3.12.1

tasks:
  install-deps:
    run: npm install && pip install -r requirements.txt

baihu:
  envs:
    - name: APP_PORT
      value: "8080"
      remark: "Application port"
  tasks:
    - name: "Start Server"
      command: "python main.py"
      schedule: "0 0 * * *"
      tools: ["python"]
      path: "main.py"
`

	service := NewManifestService()
	manifest, err := service.Parse([]byte(yamlContent))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	fmt.Printf("Parsed Tools: %+v\n", manifest.Tools)
	fmt.Printf("Parsed Baihu Envs: %+v\n", manifest.Baihu.Envs)
	fmt.Printf("Parsed Baihu Tasks: %+v\n", manifest.Baihu.Tasks)

	envs := service.ExtractEnvs(manifest, "user1")
	if len(envs) != 1 || envs[0].Name != "APP_PORT" {
		t.Errorf("ExtractEnvs failed, got: %+v", envs)
	}

	if len(tasks) != 1 || tasks[0].Name != "Start Server" || tasks[0].WorkDir != "main.py" {
		t.Errorf("ExtractTasks failed, got: %+v", tasks)
	}

	scripts := service.ExtractScripts(manifest, "user1")
	if len(scripts) != 1 || scripts[0].Name != "Start Server" {
		t.Errorf("ExtractScripts failed, got: %+v", scripts)
	}

	if len(tasks[0].Languages) != 1 || tasks[0].Languages[0]["name"] != "python" {
		t.Errorf("Task language mapping failed, got: %+v", tasks[0].Languages)
	}
}
