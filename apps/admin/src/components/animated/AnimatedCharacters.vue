<template>
  <div class="ac-scene">
    <!-- 紫色长方形 · 最后层 -->
    <div
      ref="purpleRef"
      class="ac-body"
      :style="{
        ...bodyStyle('#6C3FF5', 1, purpleTransform),
        left: '110px',
        width: '150px',
        height: isLookingAway || isTyping || isHidingPassword ? '440px' : '400px',
        borderRadius: '10px 10px 0 0',
      }"
    >
      <div
        :class="`ac-eyes ac-eyes--slow${shakeClass}`"
        :style="{ gap: '32px', left: `${purpleFace.eyeX}px`, top: `${purpleFace.eyeY}px` }"
      >
        <EyeBall
          v-for="i in 2"
          :key="i"
          :mouse-x="mouse.x"
          :mouse-y="mouse.y"
          :is-blinking="purpleBlinking"
          :force-look-x="purpleForceX"
          :force-look-y="purpleForceY"
        />
      </div>

      <div
        :class="`ac-mouth ac-mouth--purple ${mouthClass}`"
        :style="{
          left: `${purpleFace.mouthX}px`,
          top: `${purpleFace.mouthY}px`,
          '--counter-skew': isTyping || isHidingPassword ? `skewX(${-(purplePos.bodySkew - 12)}deg)` : 'skewX(0deg)',
        }"
      />
    </div>

    <!-- 黑色长方形 · 中层（没有嘴，全靠眼睛演） -->
    <div
      ref="blackRef"
      class="ac-body"
      :style="{
        ...bodyStyle('#2D2D2D', 2, blackTransform),
        left: '240px',
        width: '120px',
        height: '310px',
        borderRadius: '8px 8px 0 0',
      }"
    >
      <div
        :class="`ac-eyes ac-eyes--slow${shakeClass}`"
        :style="{
          gap: '24px',
          left: blackEyes.left,
          top: blackEyes.top,
        }"
      >
        <EyeBall
          v-for="rotate in [-20, 20]"
          :key="rotate"
          :mouse-x="mouse.x"
          :mouse-y="mouse.y"
          :size="16"
          :pupil-size="6"
          :is-blinking="blackBlinking"
          :is-sad="loginFailed"
          :sad-rotate="rotate"
          :force-look-x="blackForceX"
          :force-look-y="blackForceY"
        />
      </div>
    </div>

    <!-- 橙色半圆 · 左前 -->
    <div
      ref="orangeRef"
      class="ac-body"
      :style="{
        ...bodyStyle('#FF9B6B', 3, plainTransform(orangePos)),
        left: '0px',
        width: '240px',
        height: '200px',
        borderRadius: '120px 120px 0 0',
      }"
    >
      <div
        :class="`ac-eyes ac-eyes--fast${shakeClass}`"
        :style="{ gap: '32px', left: `${orangeFace.eyeX}px`, top: `${orangeFace.eyeY}px` }"
      >
        <Pupil
          v-for="i in 2"
          :key="i"
          :mouse-x="mouse.x"
          :mouse-y="mouse.y"
          :force-look-x="isLookingAway || isPeeping ? -5 : undefined"
          :force-look-y="isLookingAway ? -5 : isPeeping ? -4 : undefined"
        />
      </div>

      <div
        :class="`ac-mouth ac-mouth--orange ${mouthClass}`"
        :style="{ left: `${orangeFace.mouthX}px`, top: `${orangeFace.mouthY}px` }"
      />
    </div>

    <!-- 黄色圆顶长方形 · 右前 -->
    <div
      ref="yellowRef"
      class="ac-body"
      :style="{
        ...bodyStyle('#E8D754', 4, plainTransform(yellowPos)),
        left: '310px',
        width: '140px',
        height: '230px',
        borderRadius: '70px 70px 0 0',
      }"
    >
      <div
        :class="`ac-eyes ac-eyes--fast${shakeClass}`"
        :style="{
          gap: '24px',
          left: isLookingAway ? '20px' : isPeeping ? '20px' : `${52 + yellowPos.faceX}px`,
          top: isLookingAway ? '30px' : isPeeping ? '35px' : `${40 + yellowPos.faceY}px`,
        }"
      >
        <Pupil
          v-for="i in 2"
          :key="i"
          :mouse-x="mouse.x"
          :mouse-y="mouse.y"
          :force-look-x="isLookingAway || isPeeping ? -5 : undefined"
          :force-look-y="isLookingAway ? -5 : isPeeping ? -4 : undefined"
        />
      </div>

      <!-- 一条横线，用 CSS 的 d 属性过渡成波浪（失败）或弧线（成功） -->
      <div
        :class="`ac-mouth-line${shakeClass}`"
        :style="{
          left: isLookingAway ? '15px' : isPeeping ? '10px' : `${40 + yellowPos.faceX}px`,
          top: isLookingAway ? '70px' : isPeeping ? '80px' : `${80 + yellowPos.faceY}px`,
        }"
      >
        <svg width="80" height="20" viewBox="0 0 80 20">
          <path
            :class="`ac-mouth-line__path${loginFailed ? ' ac-mouth-line__path--wavy' : loginSuccess ? ' ac-mouth-line__path--happy' : ''}`"
            stroke="#2D2D2D"
            stroke-width="3"
            fill="none"
            stroke-linecap="round"
          />
        </svg>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// 四个动画角色（移植自 React 版 animatedlogin-react，Vue 3 组合式 API 实现）。
// 姿态优先级：偷看 > 回避 > 对视 > 默认鼠标跟随；登录失败摇头垮脸，成功咧嘴。
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import EyeBall from './EyeBall.vue'
import Pupil from './Pupil.vue'

const props = withDefaults(
  defineProps<{
    isTyping?: boolean
    isPasswordFocused?: boolean
    showPassword?: boolean
    passwordLength?: number
    loginFailed?: boolean
    loginSuccess?: boolean
  }>(),
  { isTyping: false, isPasswordFocused: false, showPassword: false, passwordLength: 0, loginFailed: false, loginSuccess: false },
)

const REDUCE_MOTION =
  typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches

const ORIGIN = { faceX: 0, faceY: 0, bodySkew: 0 }

/* ---------- refs 与状态 ---------- */
const purpleRef = ref<HTMLElement | null>(null)
const blackRef = ref<HTMLElement | null>(null)
const orangeRef = ref<HTMLElement | null>(null)
const yellowRef = ref<HTMLElement | null>(null)

const mouse = reactive({ x: 0, y: 0 })
const purplePos = reactive({ ...ORIGIN })
const blackPos = reactive({ ...ORIGIN })
const orangePos = reactive({ ...ORIGIN })
const yellowPos = reactive({ ...ORIGIN })

const isLookingAtEachOther = ref(false)
const isPurplePeeking = ref(false)
const isBlackPeeking = ref(false)
/** 0 = 不摇，1/2 = 交替使用的两个动画槽（保证连续失败也能重播） */
const shakeSlot = ref(0)

const purpleBlinking = useBlink()
const blackBlinking = useBlink()

const isHidingPassword = computed(() => props.passwordLength > 0 && !props.showPassword)
const isPeeping = computed(() => props.passwordLength > 0 && props.showPassword)
const isLookingAway = computed(() => props.isPasswordFocused && !props.showPassword)

/* ---------- 随机眨眼（3–7 秒，闭眼 150ms） ---------- */
function useBlink() {
  const blinking = ref(false)
  let openTimer: ReturnType<typeof setTimeout> | undefined
  let closeTimer: ReturnType<typeof setTimeout> | undefined
  const schedule = () => {
    openTimer = setTimeout(() => {
      blinking.value = true
      closeTimer = setTimeout(() => {
        blinking.value = false
        schedule()
      }, 150)
    }, Math.random() * 4000 + 3000)
  }
  onMounted(() => {
    if (REDUCE_MOTION) return
    schedule()
  })
  onBeforeUnmount(() => {
    clearTimeout(openTimer)
    clearTimeout(closeTimer)
  })
  return blinking
}

/* ---------- 鼠标跟随（requestAnimationFrame 节流，一帧算一次） ---------- */
let rafId: number | null = null
let pendingX = 0
let pendingY = 0
let needsUpdate = false

function calculatePosition(el: HTMLElement | null, mx: number, my: number) {
  if (!el) return ORIGIN
  const rect = el.getBoundingClientRect()
  const centerX = rect.left + rect.width / 2
  // 高度的 1/3：脸在上半部分，用几何中心视线会偏低
  const centerY = rect.top + rect.height / 3
  const dx = mx - centerX
  const dy = my - centerY
  return {
    faceX: Math.max(-15, Math.min(15, dx / 20)),
    faceY: Math.max(-10, Math.min(10, dy / 30)),
    bodySkew: Math.max(-6, Math.min(6, -dx / 120)),
  }
}

function onMouseMove(e: MouseEvent) {
  pendingX = e.clientX
  pendingY = e.clientY
  needsUpdate = true
}

function tick() {
  if (needsUpdate) {
    needsUpdate = false
    mouse.x = pendingX
    mouse.y = pendingY
    Object.assign(purplePos, calculatePosition(purpleRef.value, pendingX, pendingY))
    Object.assign(blackPos, calculatePosition(blackRef.value, pendingX, pendingY))
    Object.assign(orangePos, calculatePosition(orangeRef.value, pendingX, pendingY))
    Object.assign(yellowPos, calculatePosition(yellowRef.value, pendingX, pendingY))
  }
  rafId = requestAnimationFrame(tick)
}

onMounted(() => {
  if (REDUCE_MOTION) return
  window.addEventListener('mousemove', onMouseMove, { passive: true })
  rafId = requestAnimationFrame(tick)
})
onBeforeUnmount(() => {
  window.removeEventListener('mousemove', onMouseMove)
  if (rafId !== null) cancelAnimationFrame(rafId)
})

/* ---------- 表情联动 ---------- */

// 账号框聚焦时对视 0.8 秒就收——一直对视会显得呆
watch(
  () => props.isTyping,
  (typing) => {
    if (!typing) {
      isLookingAtEachOther.value = false
      return
    }
    isLookingAtEachOther.value = true
    const timer = setTimeout(() => (isLookingAtEachOther.value = false), 800)
    return () => clearTimeout(timer)
  },
)

// 密码明文时，紫色隔 2–5 秒偷瞄一次；依赖 passwordLength：停手之后才瞄
watch(
  [isPeeping, () => props.passwordLength],
  ([peeping]) => {
    if (!peeping) {
      isPurplePeeking.value = false
      return
    }
    let backTimer: ReturnType<typeof setTimeout> | undefined
    const peekTimer = setTimeout(() => {
      isPurplePeeking.value = true
      backTimer = setTimeout(() => (isPurplePeeking.value = false), 800)
    }, Math.random() * 3000 + 2000)
    return () => {
      clearTimeout(peekTimer)
      clearTimeout(backTimer)
    }
  },
)

// 密码框聚焦期间，黑色身体转开，每隔 2–5 秒往右瞟一眼（指缝里偷看）
watch(
  isLookingAway,
  (away) => {
    if (!away || REDUCE_MOTION) {
      isBlackPeeking.value = false
      return
    }
    let peekTimer: ReturnType<typeof setTimeout> | undefined
    let backTimer: ReturnType<typeof setTimeout> | undefined
    let cancelled = false
    const schedule = () => {
      peekTimer = setTimeout(() => {
        if (cancelled) return
        isBlackPeeking.value = true
        backTimer = setTimeout(() => {
          if (cancelled) return
          isBlackPeeking.value = false
          schedule()
        }, 900)
      }, Math.random() * 3000 + 2000)
    }
    schedule()
    return () => {
      cancelled = true
      clearTimeout(peekTimer)
      clearTimeout(backTimer)
    }
  },
)

// 登录失败摇头：两个动画名交替使用，保证连续失败也能重播
watch(
  () => props.loginFailed,
  (failed) => {
    if (!failed || REDUCE_MOTION) {
      shakeSlot.value = 0
      return
    }
    shakeSlot.value = shakeSlot.value === 1 ? 2 : 1
    const timer = setTimeout(() => (shakeSlot.value = 0), 800)
    return () => clearTimeout(timer)
  },
)

/* ---------- 坐标与样式 ---------- */

// 嘴跟着眼睛走：水平居中对齐眼睛，垂直下移固定距离
function faceLayout(eyeX: number, eyeY: number, eyesWidth: number, mouthWidth: number, mouthDrop: number) {
  return {
    eyeX,
    eyeY,
    mouthX: eyeX + (eyesWidth - mouthWidth) / 2,
    mouthY: eyeY + mouthDrop,
  }
}

const purpleFace = computed(() => {
  if (isLookingAway.value) return faceLayout(20, 25, 68, 24, 32)
  if (isPeeping.value) return faceLayout(20, 35, 68, 24, 32)
  if (isLookingAtEachOther.value) return faceLayout(55, 65, 68, 24, 32)
  return faceLayout(30 + purplePos.faceX, 40 + purplePos.faceY, 68, 24, 32)
})

const orangeFace = computed(() => {
  if (isLookingAway.value) return faceLayout(50, 75, 56, 26, 32)
  if (isPeeping.value) return faceLayout(50, 85, 56, 26, 32)
  return faceLayout(82 + orangePos.faceX, 90 + orangePos.faceY, 56, 26, 32)
})

const purpleForceX = computed(() =>
  isLookingAway.value ? -5 : isPeeping.value ? (isPurplePeeking.value ? 4 : -4) : isLookingAtEachOther.value ? 3 : undefined,
)
const purpleForceY = computed(() =>
  isLookingAway.value ? -5 : isPeeping.value ? (isPurplePeeking.value ? 5 : -4) : isLookingAtEachOther.value ? 4 : undefined,
)
const blackForceX = computed(() =>
  isLookingAway.value ? (isBlackPeeking.value ? 5 : -4) : isPeeping.value ? -4 : isLookingAtEachOther.value ? 0 : undefined,
)
const blackForceY = computed(() =>
  isLookingAway.value ? (isBlackPeeking.value ? 0 : -5) : isPeeping.value ? -4 : isLookingAtEachOther.value ? -4 : undefined,
)

const blackEyes = computed(() => ({
  left: isLookingAway.value
    ? '10px'
    : isPeeping.value
    ? '10px'
    : isLookingAtEachOther.value
    ? '32px'
    : `${26 + blackPos.faceX}px`,
  top: isLookingAway.value
    ? '20px'
    : isPeeping.value
    ? '28px'
    : isLookingAtEachOther.value
    ? '12px'
    : `${32 + blackPos.faceY}px`,
}))

const mouthClass = computed(() =>
  [
    props.loginFailed ? 'is-sad' : '',
    props.loginSuccess && !props.loginFailed ? 'is-happy' : '',
    (props.isTyping || isHidingPassword.value) && !props.loginFailed && !props.loginSuccess ? 'is-typing' : '',
  ]
    .filter(Boolean)
    .join(' '),
)

const shakeClass = computed(() =>
  shakeSlot.value === 1 ? ' ac-shake-a' : shakeSlot.value === 2 ? ' ac-shake-b' : '',
)

function bodyStyle(backgroundColor: string, zIndex: number, transform: string) {
  // 跟随鼠标要跟手（0.1s），换姿势要有过程（0.6s）——一律长时长会拖出糊影
  const speed = props.isPasswordFocused || props.isTyping ? '0.6s' : '0.1s'
  return {
    position: 'absolute' as const,
    backgroundColor,
    zIndex,
    transformOrigin: 'bottom center',
    willChange: 'transform',
    backfaceVisibility: 'hidden' as const,
    transition: `transform ${speed} ease-out, height 0.6s ease-in-out`,
    transform,
    // 角色倾斜时底边露缝：往下压 2px 再用同色描边补上
    bottom: '-2px',
    borderBottom: `4px solid ${backgroundColor}`,
  }
}

// 橙色和黄色只有「偷看时站直」和「跟随鼠标」两种
function plainTransform(pos: { bodySkew: number }) {
  return isPeeping.value ? 'skewX(0deg) translateZ(0)' : `skewX(${pos.bodySkew}deg) translateZ(0)`
}

const purpleTransform = computed(() =>
  isPeeping.value
    ? 'skewX(0deg) translateZ(0)'
    : isLookingAway.value
    ? 'skewX(-14deg) translateX(-20px) translateZ(0)'
    : props.isTyping || isHidingPassword.value
    ? `skewX(${purplePos.bodySkew - 12}deg) translateX(40px) translateZ(0)`
    : `skewX(${purplePos.bodySkew}deg) translateZ(0)`,
)

const blackTransform = computed(() =>
  isPeeping.value
    ? 'skewX(0deg) translateZ(0)'
    : isLookingAway.value
    ? 'skewX(12deg) translateX(-10px) translateZ(0)'
    : isLookingAtEachOther.value
    ? `skewX(${blackPos.bodySkew * 1.5 + 10}deg) translateX(20px) translateZ(0)`
    : `skewX(${blackPos.bodySkew * 1.5}deg) translateZ(0)`,
)
</script>

<style>
/* 动画角色样式（ac- 前缀，全局作用以支持嵌套子组件） */
.ac-scene {
  position: relative;
  width: 550px;
  /* 450 = 角色最高 440 + 10px 透明余量，防止眼睛被 overflow 裁掉 */
  height: 450px;
  overflow: hidden;
}

.ac-body {
  position: absolute;
  transform-origin: bottom center;
  will-change: transform;
  backface-visibility: hidden;
}

.ac-eyes {
  position: absolute;
  display: flex;
}

/* 主角转头要有过程；前景橙黄慢半拍会穿帮 */
.ac-eyes--slow {
  transition: all 0.6s ease-out;
}

.ac-eyes--fast {
  transition: all 0.2s ease-out;
}

.ac-eyeball {
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-radius: 50%;
  transition: height 0.15s ease, border-radius 0.3s ease, transform 0.3s ease;
}

.ac-pupil {
  border-radius: 50%;
  transition: transform 0.1s ease-out;
}

/* 半圆形的嘴：默认下弯（微笑），is-sad 翻成上弯（撇嘴） */
.ac-mouth {
  position: absolute;
  background-color: #2d2d2d;
  transition:
    left 0.6s ease-out,
    top 0.6s ease-out,
    width 0.5s cubic-bezier(0.4, 0, 0.2, 1),
    height 0.5s cubic-bezier(0.4, 0, 0.2, 1),
    border-radius 0.5s cubic-bezier(0.4, 0, 0.2, 1),
    transform 0.5s cubic-bezier(0.4, 0, 0.2, 1);
}

.ac-mouth--purple {
  width: 24px;
  height: 8px;
  border-radius: 0 0 12px 12px;
}

/* 抿成一竖，counter-skew 抵消身体倾斜 */
.ac-mouth--purple.is-typing {
  width: 7px;
  height: 32px;
  border-radius: 0;
  transform: translateX(8.5px) translateY(-28px) var(--counter-skew, skewX(0deg));
}

.ac-mouth--purple.is-sad {
  border-radius: 12px 12px 0 0;
}

.ac-mouth--purple.is-happy {
  width: 30px;
  height: 16px;
  border-radius: 0 0 15px 15px;
}

.ac-mouth--orange {
  width: 26px;
  height: 13px;
  border-radius: 0 0 13px 13px;
  transition:
    left 0.2s ease-out,
    top 0.2s ease-out,
    width 0.5s cubic-bezier(0.4, 0, 0.2, 1),
    height 0.5s cubic-bezier(0.4, 0, 0.2, 1),
    border-radius 0.5s cubic-bezier(0.4, 0, 0.2, 1),
    transform 0.5s cubic-bezier(0.4, 0, 0.2, 1);
}

.ac-mouth--orange.is-typing {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  transform: translateX(6px);
}

.ac-mouth--orange.is-sad {
  border-radius: 13px 13px 0 0;
}

.ac-mouth--orange.is-happy {
  width: 32px;
  height: 18px;
  border-radius: 0 0 16px 16px;
}

.ac-mouth-line {
  position: absolute;
  transition: all 0.2s ease-out;
}

/* 用 CSS 的 d 属性过渡 SVG 路径形状，比换 path 字符串平滑 */
.ac-mouth-line__path {
  d: path('M0 10 Q10 10, 20 10 Q30 10, 40 10 Q50 10, 60 10 Q70 10, 80 10');
  transition: d 0.5s cubic-bezier(0.4, 0, 0.2, 1);
}

.ac-mouth-line__path--wavy {
  d: path('M0 10 Q10 2, 20 10 Q30 18, 40 10 Q50 2, 60 10 Q70 18, 80 10');
}

.ac-mouth-line__path--happy {
  d: path('M0 2 Q10 10, 20 14 Q30 18, 40 18 Q50 18, 60 14 Q70 10, 80 2');
}

/* 摇头：幅度递减的左右摆动。两份相同动画交替挂载，强制重播 */
.ac-shake-a {
  animation: ac-shake-head-a 0.8s cubic-bezier(0.36, 0.07, 0.19, 0.97) both;
}

.ac-shake-b {
  animation: ac-shake-head-b 0.8s cubic-bezier(0.36, 0.07, 0.19, 0.97) both;
}

@keyframes ac-shake-head-a {
  0%, 100% { translate: 0 0; }
  10% { translate: -9px 0; }
  20% { translate: 7px 0; }
  30% { translate: -6px 0; }
  40% { translate: 5px 0; }
  50% { translate: -4px 0; }
  60% { translate: 3px 0; }
  70% { translate: -2px 0; }
  80% { translate: 1px 0; }
  90% { translate: -0.5px 0; }
}

@keyframes ac-shake-head-b {
  0%, 100% { translate: 0 0; }
  10% { translate: -9px 0; }
  20% { translate: 7px 0; }
  30% { translate: -6px 0; }
  40% { translate: 5px 0; }
  50% { translate: -4px 0; }
  60% { translate: 3px 0; }
  70% { translate: -2px 0; }
  80% { translate: 1px 0; }
  90% { translate: -0.5px 0; }
}

/* 无障碍：系统关闭动效时保持静态姿势 */
@media (prefers-reduced-motion: reduce) {
  .ac-body,
  .ac-eyes,
  .ac-pupil,
  .ac-eyeball,
  .ac-mouth,
  .ac-mouth-line,
  .ac-mouth-line__path {
    transition: none !important;
    animation: none !important;
  }
}
</style>
