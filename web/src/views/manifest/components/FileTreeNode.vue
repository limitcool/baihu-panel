<script setup lang="ts">
import { type FileNode } from '@/api'
import { Folder, File, ChevronRight, ChevronDown } from 'lucide-vue-next'
import { Checkbox } from '@/components/ui/checkbox'

const props = withDefaults(defineProps<{
  node: FileNode
  selectedPaths: Set<string>
  expandedDirs: Set<string>
  depth?: number
}>(), {
  depth: 0
})

const emit = defineEmits(['toggle', 'toggle-dir'])
</script>

<template>
  <div class="select-none">
    <div 
      class="flex items-center gap-1.5 py-1 px-2 rounded-md hover:bg-muted/60 cursor-pointer group transition-all"
      :style="{ paddingLeft: (node.isDir ? depth * 12 + 4 : depth * 12 + 20) + 'px' }">
      
      <template v-if="node.isDir">
        <div @click.stop="emit('toggle-dir', node.path)" class="p-0.5 hover:bg-muted rounded transition-colors text-muted-foreground/70 group-hover:text-foreground">
          <ChevronRight v-if="!expandedDirs.has(node.path)" class="h-3 w-3" />
          <ChevronDown v-else class="h-3 w-3" />
        </div>
        <Folder class="h-3.5 w-3.5 text-yellow-500/90 fill-yellow-500/20" />
      </template>
      <File v-else class="h-3.5 w-3.5 text-blue-500/80" />
      
      <div class="flex-1 min-w-0 text-[13px] truncate" @click="emit('toggle', node)">
        {{ node.name }}
      </div>
      
      <Checkbox 
        :checked="selectedPaths.has(node.path)" 
        @update:checked="() => emit('toggle', node)"
        class="h-3.5 w-3.5 data-[state=checked]:bg-primary data-[state=checked]:border-primary" />
    </div>
    
    <div v-if="node.isDir && expandedDirs.has(node.path)" class="mt-0.5">
      <FileTreeNode 
        v-for="child in node.children" 
        :key="child.path" 
        :node="child" 
        :selected-paths="selectedPaths"
        :expanded-dirs="expandedDirs"
        :depth="depth + 1"
        @toggle="emit('toggle', $event)"
        @toggle-dir="emit('toggle-dir', $event)"
      />
    </div>
  </div>
</template>
