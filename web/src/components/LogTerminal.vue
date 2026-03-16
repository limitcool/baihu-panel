<script setup lang="ts">
import { ref, onMounted, nextTick, watch } from 'vue'
import Ansi from 'ansi-to-vue3'

const props = withDefaults(
  defineProps<{
    content: string
    fontSize?: number
    theme?: 'dark' | 'light'
    autoScroll?: boolean
  }>(),
  {
    fontSize: 12,
    theme: 'dark',
    autoScroll: true
  }
)

const scrollContainerRef = ref<HTMLDivElement | null>(null)
const lightBackgroundClass = 'bg-zinc-100 text-zinc-900'
const darkBackgroundClass = 'bg-zinc-950 text-zinc-300' // 稍微调亮一点暗色模式文字

// 自动滚动到底部逻辑
const scrollToBottom = () => {
  if (props.autoScroll && scrollContainerRef.value) {
    scrollContainerRef.value.scrollTop = scrollContainerRef.value.scrollHeight
  }
}

watch(() => props.content, () => {
  if (props.autoScroll) {
    nextTick(scrollToBottom)
  }
})

onMounted(() => {
  if (props.autoScroll) {
    setTimeout(scrollToBottom, 50)
  }
})

defineExpose({
  fit: () => { }, 
  clear: () => { },
  scrollToBottom
})
</script>

<template>
  <div
    ref="scrollContainerRef"
    class="w-full h-full overflow-y-auto overflow-x-auto log-container native-scroll"
    :class="theme === 'dark' ? darkBackgroundClass : lightBackgroundClass"
    :style="{ fontSize: fontSize + 'px' }"
  ><div class="inline-block min-w-full p-3 font-mono leading-normal whitespace-pre text-left"><Ansi>{{ content }}</Ansi></div></div>
</template>

<style scoped>
.native-scroll {
  scrollbar-gutter: stable;
  -webkit-overflow-scrolling: touch;
  overscroll-behavior-y: contain;
}

/* 强制覆盖 ansi-to-vue3 可能生成的 code 样式 */
:deep(code) {
  background: transparent !important;
  font-family: inherit;
  padding: 0 !important;
  margin: 0 !important;
}

/* 滚动条美化 */
.native-scroll::-webkit-scrollbar {
  width: 6px;
  height: 6px;
}

.native-scroll::-webkit-scrollbar-thumb {
  background: rgba(128, 128, 128, 0.2);
  border-radius: 10px;
}

.native-scroll::-webkit-scrollbar-thumb:hover {
  background: rgba(128, 128, 128, 0.4);
}

.native-scroll::-webkit-scrollbar-track {
  background: transparent;
}
</style>
