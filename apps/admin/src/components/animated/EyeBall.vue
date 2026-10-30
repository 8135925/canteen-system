<template>
  <div
    ref="el"
    class="ac-eyeball"
    :style="{
      width: `${size}px`,
      height: isBlinking ? '2px' : isSad ? `${size * 0.5}px` : `${size}px`,
      backgroundColor: eyeColor,
      borderRadius: isSad ? `0 0 ${size}px ${size}px` : '50%',
      transform: isSad ? `rotate(${sadRotate}deg)` : 'rotate(0deg)',
    }"
  >
    <div
      v-if="!isBlinking"
      class="ac-pupil"
      :style="{
        width: `${pupilSize}px`,
        height: `${pupilSize}px`,
        backgroundColor: pupilColor,
        transform: `translate(${pos.x}px, ${isSad ? -1 : pos.y}px)`,
      }"
    />
  </div>
</template>

<script setup lang="ts">
// 带眼白的眼球（紫色、黑色角色）。
// isSad 把整只眼压扁成半圆并旋转；左右眼传相反的 sadRotate 合成垂眼。
import { computed, onMounted, ref, watch } from 'vue'

const props = withDefaults(
  defineProps<{
    mouseX: number
    mouseY: number
    size?: number
    pupilSize?: number
    maxDistance?: number
    eyeColor?: string
    pupilColor?: string
    isBlinking?: boolean
    forceLookX?: number
    forceLookY?: number
    isSad?: boolean
    sadRotate?: number
  }>(),
  { size: 18, pupilSize: 7, maxDistance: 5, eyeColor: '#fff', pupilColor: '#2D2D2D', isBlinking: false, isSad: false, sadRotate: 0 },
)

const el = ref<HTMLElement | null>(null)
const pos = ref({ x: 0, y: 0 })

function update() {
  if (props.forceLookX !== undefined && props.forceLookY !== undefined) {
    pos.value = { x: props.forceLookX, y: props.forceLookY }
    return
  }
  if (!el.value) return
  // 每次实测：角色身体在倾斜变形，眼睛的屏幕位置一直在变，缓存会错位
  const rect = el.value.getBoundingClientRect()
  const dx = props.mouseX - (rect.left + rect.width / 2)
  const dy = props.mouseY - (rect.top + rect.height / 2)
  const distance = Math.min(Math.sqrt(dx * dx + dy * dy), props.maxDistance)
  const angle = Math.atan2(dy, dx)
  pos.value = { x: Math.cos(angle) * distance, y: Math.sin(angle) * distance }
}

watch(
  [() => props.mouseX, () => props.mouseY, () => props.forceLookX, () => props.forceLookY],
  update,
)
onMounted(update)

// 消除未使用告警（computed 依赖收集用）
void computed
</script>
