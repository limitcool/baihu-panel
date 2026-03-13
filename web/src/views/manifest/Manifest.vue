<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Input } from '@/components/ui/input'
import { api, type FileNode, type EnvVar, type MiseLanguage } from '@/api'
import { FileDown, Folder, Variable, Globe, Copy, X, Search, RefreshCw, Eye } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import FileTreeNode from './components/FileTreeNode.vue'

const fileTree = ref<FileNode[]>([])
const fileSearch = ref('')
const envs = ref<EnvVar[]>([])
const envSearch = ref('')
const languages = ref<MiseLanguage[]>([])
const langSearch = ref('')

const selectedPaths = ref<Set<string>>(new Set())
const selectedEnvs = ref<Set<string>>(new Set())
const selectedLangs = ref<Set<string>>(new Set())

const expandedDirs = ref<Set<string>>(new Set())

const loading = ref(false)

async function loadData() {
  loading.value = true
  try {
    const [treeRes, envRes, langRes] = await Promise.all([
      api.files.tree(),
      api.env.all(),
      api.mise.list()
    ])
    fileTree.value = treeRes
    envs.value = envRes || []
    languages.value = langRes || []
  } catch (err) {
    toast.error('获取数据失败')
  } finally {
    loading.value = false
  }
}

// Filtered lists
const filteredEnvs = computed(() => {
  if (!envSearch.value) return envs.value
  return envs.value.filter(e => e.name.toLowerCase().includes(envSearch.value.toLowerCase()))
})

const filteredLangs = computed(() => {
  if (!langSearch.value) return languages.value
  return languages.value.filter(l => l.plugin.toLowerCase().includes(langSearch.value.toLowerCase()))
})

// Recursive filter for file tree (simplified for now, just filtering top level or children if they match)
const filteredFileTree = computed(() => {
  if (!fileSearch.value) return fileTree.value
  const search = fileSearch.value.toLowerCase()
  
  function filterTree(nodes: FileNode[]): FileNode[] {
    return nodes.filter(node => {
      if (node.name.toLowerCase().includes(search)) return true
      if (node.children) {
        const filteredChildren = filterTree(node.children)
        if (filteredChildren.length > 0) {
          // If children match, we might want to keep the parent expanded
          expandedDirs.value.add(node.path)
          return true
        }
      }
      return false
    })
  }
  
  return filterTree(fileTree.value)
})

function toggleDir(path: string) {
  if (expandedDirs.value.has(path)) {
    expandedDirs.value.delete(path)
  } else {
    expandedDirs.value.add(path)
  }
}

function togglePath(node: FileNode) {
  if (selectedPaths.value.has(node.path)) {
    selectedPaths.value.delete(node.path)
  } else {
    selectedPaths.value.add(node.path)
  }
}

function toggleEnv(id: string) {
  if (selectedEnvs.value.has(id)) {
    selectedEnvs.value.delete(id)
  } else {
    selectedEnvs.value.add(id)
  }
}

function toggleLang(plugin: string, version: string) {
  const identifier = `${plugin}@${version}`
  if (selectedLangs.value.has(identifier)) {
    selectedLangs.value.delete(identifier)
  } else {
    selectedLangs.value.add(identifier)
  }
}

const showYamlPreview = ref(false)
const yamlContent = ref('')

function toYaml(obj: any): string {
  let yaml = ''
  if (obj.tools && Object.keys(obj.tools).length > 0) {
    yaml += 'tools:\n'
    for (const [k, v] of Object.entries(obj.tools)) {
      yaml += `  ${k}: ${v}\n`
    }
  }
  if (obj.baihu) {
    if (yaml) yaml += '\n'
    yaml += 'baihu:\n'
    if (obj.baihu.envs && obj.baihu.envs.length > 0) {
      yaml += '  envs:\n'
      obj.baihu.envs.forEach((e: any) => {
        yaml += `    - name: ${e.name}\n`
        yaml += `      value: "${e.value}"\n`
        if (e.remark) yaml += `      remark: "${e.remark}"\n`
        yaml += `      enabled: ${e.enabled}\n`
      })
    }
    if (obj.baihu.tasks && obj.baihu.tasks.length > 0) {
      if (obj.baihu.envs && obj.baihu.envs.length > 0) yaml += '\n'
      yaml += '  tasks:\n'
      obj.baihu.tasks.forEach((t: any) => {
        yaml += `    - name: "${t.name}"\n`
        yaml += `      command: "${t.command}"\n`
        yaml += `      path: "${t.path}"\n`
        yaml += `      enabled: ${t.enabled}\n`
      })
    }
  }
  return yaml || '# 请选择要导出的内容'
}

const exporting = ref(false)

async function downloadZip() {
  if (selectedPaths.value.size === 0 && selectedEnvs.value.size === 0 && selectedLangs.value.size === 0) {
    toast.warning('请至少选择一个项进行导出')
    return
  }
  
  exporting.value = true
  try {
    const data = {
      paths: Array.from(selectedPaths.value),
      tools: {} as Record<string, string>,
      envs: [] as any[],
      tasks: [] as any[]
    }

    selectedLangs.value.forEach(id => {
      const [plugin, version] = id.split('@')
      data.tools[plugin] = version
    })
    
    selectedEnvs.value.forEach(id => {
      const env = envs.value.find(e => e.id === id)
      if (env) data.envs.push({ name: env.name, value: env.value, remark: env.remark, enabled: env.enabled })
    })

    selectedPaths.value.forEach(path => {
      const name = path.split('/').pop() || path
      data.tasks.push({
        name: `运行 ${name}`,
        command: path.endsWith('.py') ? `python ${path}` : path.endsWith('.js') ? `node ${path}` : `./${path}`,
        path: path,
        enabled: true
      })
    })

    const blob = await api.manifest.export(data)
    const url = window.URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `project_${new Date().toISOString().split('T')[0]}.zip`
    document.body.appendChild(a)
    a.click()
    window.URL.revokeObjectURL(url)
    document.body.removeChild(a)
    toast.success('导出成功')
  } catch (err) {
    toast.error('导出失败: ' + (err instanceof Error ? err.message : String(err)))
  } finally {
    exporting.value = false
  }
}

function handlePreview() {
  if (selectedPaths.value.size === 0 && selectedEnvs.value.size === 0 && selectedLangs.value.size === 0) {
    toast.warning('请至少选择一个项进行预览')
    return
  }
  const manifest: any = { tools: {}, baihu: { envs: [], tasks: [] } }
  selectedLangs.value.forEach(id => {
    const [plugin, version] = id.split('@')
    manifest.tools[plugin] = version
  })
  selectedEnvs.value.forEach(id => {
    const env = envs.value.find(e => e.id === id)
    if (env) manifest.baihu.envs.push({ name: env.name, value: env.value, remark: env.remark, enabled: env.enabled })
  })
  selectedPaths.value.forEach(path => {
    const name = path.split('/').pop() || path
    manifest.baihu.tasks.push({
      name: `运行 ${name}`,
      command: path.endsWith('.py') ? `python ${path}` : path.endsWith('.js') ? `node ${path}` : `./${path}`,
      path: path,
      enabled: true
    })
  })
  yamlContent.value = toYaml(manifest)
  showYamlPreview.value = true
}

function copyYaml() {
  navigator.clipboard.writeText(yamlContent.value)
  toast.success('已复制到剪贴板')
}

function getLangIcon(plugin: string) {
  const name = plugin.toLowerCase().trim()
  const mapping: Record<string, string> = {
    'python': 'python/python-original.svg',
    'node': 'nodejs/nodejs-original.svg',
    'nodejs': 'nodejs/nodejs-original.svg',
    'go': 'go/go-original.svg',
    'rust': 'rust/rust-original.svg',
    'ruby': 'ruby/ruby-plain.svg',
    'php': 'php/php-plain.svg',
    'java': 'java/java-plain.svg',
    'deno': 'deno/deno-plain.svg',
    'bun': 'bun/bun-plain.svg',
    'zig': 'zig/zig-original.svg',
    'dotnet': 'dot-net/dot-net-original.svg',
    '.net': 'dot-net/dot-net-original.svg',
    'elixir': 'elixir/elixir-original.svg',
    'erlang': 'erlang/erlang-original.svg',
    'crystal': 'crystal/crystal-original.svg',
    'lua': 'lua/lua-original.svg',
    'julia': 'julia/julia-original.svg',
    'nim': 'nim/nim-original.svg',
    'perl': 'perl/perl-original.svg',
    'scala': 'scala/scala-original.svg',
    'kotlin': 'kotlin/kotlin-original.svg',
    'clojure': 'clojure/clojure-line.svg',
    'dart': 'dart/dart-original.svg',
    'flutter': 'flutter/flutter-original.svg',
    'terraform': 'terraform/terraform-original.svg',
    'docker': 'docker/docker-original.svg',
    'kubernetes': 'kubernetes/kubernetes-plain.svg',
    'ansible': 'ansible/ansible-original.svg',
  }
  return mapping[name] ? `https://fastly.jsdelivr.net/gh/devicons/devicon/icons/${mapping[name]}` : ''
}

onMounted(loadData)
</script>

<template>
  <div class="h-[calc(100vh-100px)] flex flex-col space-y-4">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div>
        <h2 class="text-xl sm:text-2xl font-bold tracking-tight">项目导出导入</h2>
        <p class="text-muted-foreground text-sm">打包分发项目，包含脚本、环境及版本信息</p>
      </div>
      <div class="flex gap-2 shrink-0">
        <Button variant="outline" size="sm" class="h-9 gap-2" @click="loadData" :disabled="loading">
          <RefreshCw class="h-4 w-4" :class="{ 'animate-spin': loading }" /> 刷新
        </Button>
        <Button variant="outline" size="sm" class="h-9 gap-2" @click="handlePreview">
          <Eye class="h-4 w-4" /> 预览配置
        </Button>
        <Button @click="downloadZip" size="sm" class="h-9 gap-2 bg-primary" :disabled="exporting">
          <FileDown v-if="!exporting" class="h-4 w-4" />
          <RefreshCw v-else class="h-4 w-4 animate-spin" />
          {{ exporting ? '导出中...' : '打包并导出' }}
        </Button>
      </div>
    </div>

    <!-- Triple Tree Layout -->
    <div class="flex flex-col lg:flex-row flex-1 min-h-0 border rounded-xl bg-card/30 overflow-hidden shadow-sm divide-y lg:divide-y-0 lg:divide-x">
      <!-- 1. Scripts Tree -->
      <div class="flex flex-col flex-1 min-w-0 h-full overflow-hidden">
        <div class="p-3 bg-muted/30 flex items-center justify-between border-b">
          <div class="flex items-center gap-2 font-semibold text-sm">
            <Folder class="h-4 w-4 text-yellow-500 fill-yellow-500/20" /> 脚本路径
          </div>
          <div class="relative w-32">
            <Search class="absolute left-1.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
            <Input v-model="fileSearch" placeholder="搜索文件..." class="h-8 pl-8 pr-1 text-[11px] rounded-lg bg-background/50 border-none shadow-inner" />
          </div>
        </div>
        <div class="flex-1 overflow-hidden p-2">
          <ScrollArea class="h-full">
            <div class="space-y-0.5 pr-6">
              <div v-if="loading && fileTree.length === 0" class="py-12 flex flex-col items-center justify-center gap-3 opacity-40">
                <RefreshCw class="h-8 w-8 animate-spin" />
                <span class="text-xs font-medium">深度扫描文件树...</span>
              </div>
              <div v-else-if="filteredFileTree.length === 0" class="py-12 text-center text-xs text-muted-foreground opacity-40 italic">
                未找到匹配的代码脚本
              </div>
              <FileTreeNode 
                v-for="node in filteredFileTree" 
                :key="node.path" 
                :node="node" 
                :selected-paths="selectedPaths"
                :expanded-dirs="expandedDirs"
                @toggle="togglePath"
                @toggle-dir="toggleDir"
              />
            </div>
          </ScrollArea>
        </div>
      </div>

      <!-- 2. Environments Tree -->
      <div class="flex flex-col flex-1 min-w-0 h-full overflow-hidden">
        <div class="p-3 bg-muted/30 flex items-center justify-between border-b">
          <div class="flex items-center gap-2 font-semibold text-sm">
            <Variable class="h-4 w-4 text-green-600" /> 环境变量
          </div>
          <div class="relative w-32">
            <Search class="absolute left-1.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
            <Input v-model="envSearch" placeholder="搜索变量..." class="h-8 pl-8 pr-1 text-[11px] rounded-lg bg-background/50 border-none shadow-inner" />
          </div>
        </div>
        <div class="flex-1 overflow-hidden p-2">
          <ScrollArea class="h-full">
            <div class="space-y-1 pr-6">
              <div v-if="filteredEnvs.length === 0" class="py-12 text-center text-xs text-muted-foreground opacity-40 italic">
                {{ loading ? '同步加载中...' : '当前环境暂无可用变量' }}
              </div>
              <div v-for="env in filteredEnvs" :key="env.id" 
                @click="toggleEnv(env.id)"
                class="flex items-center gap-2.5 py-1.5 px-3 rounded-lg hover:bg-muted/80 cursor-pointer group transition-all duration-200">
                <Variable class="h-3.5 w-3.5 text-green-500/70 shrink-0" />
                <div class="flex-1 min-w-0">
                  <div class="text-[13px] font-mono font-medium truncate group-hover:text-primary transition-colors">{{ env.name }}</div>
                  <div class="text-[10px] text-muted-foreground truncate opacity-70">{{ env.remark || '未设置备注说明' }}</div>
                </div>
                <Checkbox 
                  :checked="selectedEnvs.has(env.id)" 
                  @update:checked="() => toggleEnv(env.id)"
                  class="h-4 w-4 data-[state=checked]:bg-primary data-[state=checked]:border-primary rounded" />
              </div>
            </div>
          </ScrollArea>
        </div>
      </div>

      <!-- 3. Languages Tree -->
      <div class="flex flex-col flex-1 min-w-0 h-full overflow-hidden">
        <div class="p-3 bg-muted/30 flex items-center justify-between border-b">
          <div class="flex items-center gap-2 font-semibold text-sm">
            <Globe class="h-4 w-4 text-purple-600" /> 运行环境
          </div>
          <div class="relative w-32">
            <Search class="absolute left-1.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
            <Input v-model="langSearch" placeholder="搜索版本..." class="h-8 pl-8 pr-1 text-[11px] rounded-lg bg-background/50 border-none shadow-inner" />
          </div>
        </div>
        <div class="flex-1 overflow-hidden p-2">
          <ScrollArea class="h-full">
            <div class="space-y-1 pr-6">
              <div v-if="filteredLangs.length === 0" class="py-12 text-center text-xs text-muted-foreground opacity-40 italic">
                {{ loading ? '获取版本列表中...' : '未检测到已安装的运行时' }}
              </div>
              <div v-for="lang in filteredLangs" :key="`${lang.plugin}@${lang.version}`" 
                @click="toggleLang(lang.plugin, lang.version)"
                class="flex items-center gap-2.5 py-2 px-3 rounded-lg hover:bg-muted/80 cursor-pointer group transition-all duration-200">
                
                <div class="h-5 w-5 shrink-0 flex items-center justify-center p-1 bg-background rounded-md shadow-sm border border-border/40 group-hover:border-primary/50 transition-colors">
                  <img v-if="getLangIcon(lang.plugin)" :src="getLangIcon(lang.plugin)" class="w-full h-full object-contain" />
                  <Globe v-else class="h-3 w-3 text-muted-foreground" />
                </div>

                <div class="flex-1 min-w-0">
                  <div class="text-[13px] font-bold flex items-center justify-between gap-2">
                    <span class="truncate capitalize group-hover:text-primary transition-colors">{{ lang.plugin }}</span>
                    <span class="text-[10px] font-mono text-muted-foreground bg-accent/50 group-hover:bg-primary/10 group-hover:text-primary px-1.5 py-0.5 rounded transition-all italic">{{ lang.version }}</span>
                  </div>
                  <div class="text-[10px] text-muted-foreground truncate opacity-60 font-mono">{{ lang.install_path }}</div>
                </div>
                <Checkbox 
                  :checked="selectedLangs.has(`${lang.plugin}@${lang.version}`)" 
                  @update:checked="() => toggleLang(lang.plugin, lang.version)"
                  class="h-4 w-4 data-[state=checked]:bg-primary data-[state=checked]:border-primary rounded" />
              </div>
            </div>
          </ScrollArea>
        </div>
      </div>
    </div>

    <!-- YAML Preview Overlay -->
    <div v-if="showYamlPreview" class="fixed inset-0 z-[100] flex items-center justify-center p-4 sm:p-6 bg-background/60 backdrop-blur-md animate-in fade-in duration-300">
      <Card class="w-full max-w-2xl max-h-[85vh] flex flex-col shadow-2xl border-primary/10 overflow-hidden scale-in animate-in zoom-in-95 duration-200">
        <CardHeader class="flex flex-row items-center justify-between gap-4 border-b bg-card py-3 px-4 shrink-0">
          <CardTitle class="text-base sm:text-lg flex items-center gap-2">
            <Copy class="h-4 w-4 text-primary" /> 导出预览 (mise.yml)
          </CardTitle>
          <Button variant="ghost" size="icon" @click="showYamlPreview = false" class="h-8 w-8 rounded-full hover:bg-muted transition-colors">
            <X class="h-4 w-4 text-muted-foreground" />
          </Button>
        </CardHeader>
        <CardContent class="flex-1 overflow-hidden p-0 relative bg-muted/10">
          <ScrollArea class="h-full p-4 sm:p-6 font-mono text-xs sm:text-sm leading-relaxed whitespace-pre text-foreground/90 selection:bg-primary/20">
            {{ yamlContent }}
          </ScrollArea>
        </CardContent>
        <div class="p-3 border-t bg-card flex justify-end gap-2 shrink-0">
          <Button variant="outline" size="sm" @click="showYamlPreview = false">
            关闭
          </Button>
          <Button size="sm" @click="copyYaml" class="gap-2 shadow-sm">
            <Copy class="h-3.5 w-3.5" /> 复制配置
          </Button>
        </div>
      </Card>
    </div>
  </div>
</template>

<style scoped>
.scale-in {
  transform: scale(0.95);
  animation: scale-up 0.2s forwards ease-out;
}

@keyframes scale-up {
  to { transform: scale(1); }
}

.hide-scrollbar::-webkit-scrollbar {
  display: none;
}
.hide-scrollbar {
  -ms-overflow-style: none;
  scrollbar-width: none;
}
</style>
