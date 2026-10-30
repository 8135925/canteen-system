<template>
  <div ref="el" class="ac-pupil" :style="styleObj" />
</template>

<script setup lang="ts">
// 纯瞳孔（无眼白）：橙色、黄色等小体积亮底色角色使用。
// 视线 = 朝鼠标方向偏移，但距离不超过 maxDistance；forceLook 强制看向某方向（对视/回避/偷看）。
import { computed, onMounted, ref, watch } from 'vue'

const props = defineProps<{
  mouseX: number
  mouseY: number
  size?: number
  maxDistance?: number
  pupilColor?: string
  forceLookX?: number
  forceLookY?: number
}>()

const el = ref<HTMLElement | null>(null)
const pos = ref({ x: 0, y: 0 })

function update() {
  if (props.forceLookX !== undefined && props.forceLookY !== undefined) {
    pos.value = { x: props.forceLookX, y: props.forceLookY }
    return
  }
  if (!el.value) return
  const rect = el.value.getBoundingClientRect()
  const dx = props.mouseX - (rect.left + rect.width / 2)
  const dy = props.mouseY - (rect.top + rect.height / 2)
  const distance = Math.min(Math.sqrt(dx * dx + dy * dy), props.maxDistance ?? 5)
  const angle = Math.atan2(dy, dx)
  pos.value = { x: Math.cos(angle) * distance, y: Math.sin(angle) * distance }
}

watch(
  [() => props.mouseX, () => props.mouseY, () => props.forceLookX, () => props.forceLookY],
  update,
)
onMounted(update)

const styleObj = computed(() => ({
  width: `${props.size ?? 12}px`,
  height: `${props.size ?? 12}px`,
  backgroundColor: props.pupilColor ?? '#2D2D2D',
  transform: `translate(${pos.value.x}px, ${pos.value.y}px)`,
}))
</script>
