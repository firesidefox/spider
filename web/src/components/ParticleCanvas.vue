<template>
  <canvas ref="canvasRef" class="particle-canvas" />
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'

const props = defineProps<{ isDark: boolean }>()
const canvasRef = ref<HTMLCanvasElement | null>(null)

interface Particle {
  x: number; y: number
  vx: number; vy: number
}

function randomBetween(a: number, b: number) {
  return a + Math.random() * (b - a)
}

function initParticles(w: number, h: number): Particle[] {
  return Array.from({ length: 60 }, () => {
    const speed = randomBetween(0.3, 0.8)
    const angle = Math.random() * Math.PI * 2
    return {
      x: Math.random() * w,
      y: Math.random() * h,
      vx: Math.cos(angle) * speed,
      vy: Math.sin(angle) * speed,
    }
  })
}

const mouse = { x: -9999, y: -9999 }
let particles: Particle[] = []
let animId = 0

function tick(ctx: CanvasRenderingContext2D, w: number, h: number) {
  const particleColor = props.isDark
    ? 'rgba(99,179,237,0.7)' : 'rgba(49,130,206,0.6)'
  const lineBaseAlpha = props.isDark ? 0.2 : 0.15
  const lineRgb = props.isDark ? '99,179,237' : '49,130,206'

  ctx.clearRect(0, 0, w, h)

  for (const p of particles) {
    const dx = mouse.x - p.x
    const dy = mouse.y - p.y
    const dist = Math.sqrt(dx * dx + dy * dy)
    if (dist < 150) {
      p.vx += (dx / dist) * 0.02
      p.vy += (dy / dist) * 0.02
      p.vx = Math.max(-2, Math.min(2, p.vx))
      p.vy = Math.max(-2, Math.min(2, p.vy))
    }

    p.x += p.vx
    p.y += p.vy

    if (p.x < 0 || p.x > w) p.vx *= -1
    if (p.y < 0 || p.y > h) p.vy *= -1

    ctx.beginPath()
    ctx.arc(p.x, p.y, 2, 0, Math.PI * 2)
    ctx.fillStyle = particleColor
    ctx.fill()
  }

  ctx.lineWidth = 1
  for (let i = 0; i < particles.length; i++) {
    for (let j = i + 1; j < particles.length; j++) {
      const a = particles[i], b = particles[j]
      const dx = a.x - b.x, dy = a.y - b.y
      const d = Math.sqrt(dx * dx + dy * dy)
      if (d < 120) {
        const alpha = (1 - d / 120) * lineBaseAlpha
        ctx.beginPath()
        ctx.moveTo(a.x, a.y)
        ctx.lineTo(b.x, b.y)
        ctx.strokeStyle = `rgba(${lineRgb},${alpha})`
        ctx.stroke()
      }
    }
  }

  animId = requestAnimationFrame(() => tick(ctx, w, h))
}

function startAnimation() {
  const canvas = canvasRef.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  const w = canvas.offsetWidth
  const h = canvas.offsetHeight
  if (w === 0 || h === 0) return
  canvas.width = w
  canvas.height = h
  particles = initParticles(w, h)

  cancelAnimationFrame(animId)
  tick(ctx, w, h)
}

function onMouseMove(e: MouseEvent) {
  const canvas = canvasRef.value
  if (!canvas) return
  const rect = canvas.getBoundingClientRect()
  mouse.x = e.clientX - rect.left
  mouse.y = e.clientY - rect.top
}

function onResize() {
  cancelAnimationFrame(animId)
  startAnimation()
}

onMounted(() => {
  startAnimation()
  canvasRef.value?.addEventListener('mousemove', onMouseMove)
  window.addEventListener('resize', onResize)
})

onUnmounted(() => {
  cancelAnimationFrame(animId)
  canvasRef.value?.removeEventListener('mousemove', onMouseMove)
  window.removeEventListener('resize', onResize)
})

watch(() => props.isDark, () => {
  cancelAnimationFrame(animId)
  startAnimation()
})
</script>

<style scoped>
.particle-canvas {
  display: block;
  width: 100%;
  height: 100%;
}
</style>
