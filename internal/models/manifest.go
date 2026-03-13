package models

// BaihuManifest represents the structure of mise.yml with baihu extensions
type BaihuManifest struct {
	Tools map[string]string      `yaml:"tools" json:"tools"`
	Tasks map[string]MiseTask    `yaml:"tasks,omitempty" json:"tasks,omitempty"`
	Baihu BaihuNode              `yaml:"baihu" json:"baihu"`
}

// MiseTask represents a task defined in the native mise section
type MiseTask struct {
	Run   string            `yaml:"run" json:"run"`
	Shell string            `yaml:"shell,omitempty" json:"shell,omitempty"`
	Env   map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	Dir   string            `yaml:"dir,omitempty" json:"dir,omitempty"`
}

// BaihuNode contains baihu-specific distribution configurations
type BaihuNode struct {
	Envs  []BaihuEnv  `yaml:"envs,omitempty" json:"envs,omitempty"`
	Tasks []BaihuTask `yaml:"tasks,omitempty" json:"tasks,omitempty"`
}

// BaihuEnv represents an environment variable to be managed
type BaihuEnv struct {
	Name    string `yaml:"name" json:"name"`
	Value   string `yaml:"value" json:"value"`
	Remark  string `yaml:"remark,omitempty" json:"remark,omitempty"`
	Hidden  bool   `yaml:"hidden,omitempty" json:"hidden,omitempty"`
	Enabled bool   `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

// BaihuTask represents a scheduled task to be added to the panel
type BaihuTask struct {
	Name          string   `yaml:"name" json:"name"`
	Command       string   `yaml:"command" json:"command"`
	Schedule      string   `yaml:"schedule,omitempty" json:"schedule,omitempty"`
	Remark        string   `yaml:"remark,omitempty" json:"remark,omitempty"`
	Timeout       int      `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	RetryCount    int      `yaml:"retry_count,omitempty" json:"retry_count,omitempty"`
	RetryInterval int      `yaml:"retry_interval,omitempty" json:"retry_interval,omitempty"`
	RandomRange   int      `yaml:"random_range,omitempty" json:"random_range,omitempty"`
	Enabled       bool     `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Tools         []string `yaml:"tools,omitempty" json:"tools,omitempty"` // which tools from 'tools' section to use, empty for all
	Path          string   `yaml:"path,omitempty" json:"path,omitempty"`   // script file path for distribution management
}
