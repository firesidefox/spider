# Login Page Particle Effect Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an AI-themed neural-network particle animation to the login page background.

**Architecture:** A self-contained `ParticleCanvas.vue` component renders a Canvas 2D animation of 60 floating nodes with distance-based connecting lines and gentle mouse attraction. `LoginView.vue` mounts it as an absolutely-positioned background layer behind the login card.

**Tech Stack:** Vue 3 Composition API, Canvas 2D API, `requestAnimationFrame`, no external dependencies.

---

### Task 1: Create ParticleCanvas.vue — template + props

**Files:**
- Create: `web/src/components/ParticleCanvas.vue`

- [ ] **Step 1: Create the file with template and props scaffold**

```vue
<template>
  <canvas ref="canvasRef" class="particle-canvas" />
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'

const props = defineProps<{ isDark: boolean }>()
const canvasRef = ref<HTMLCanvasElement | null>(null)
</script>

<style scoped>
.particle-canvas {
  display: block;
  width: 100%;
  height: 100%;
}
</style>
```

- [ ] **Step 2: Commit scaffold**

```bash
git add web/src/components/ParticleCanvas.vue
git commit -m "feat: scaffold ParticleCanvas component"
```

---

### Task 2: Implement particle system logic

**Files:**
- Modify: `web/src/components/ParticleCanvas.vue`

- [ ] **Step 1: Add particle type and init function after the props line**

```typescript
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
```

- [ ] **Step 2: Add mouse tracking and animation state**

```typescript
const mouse = { x: -9999, y: -9999 }
let particles: Particle[] = []
let animId = 0
```

- [ ] **Step 3: Add update + draw function**

```typescript
function tick(ctx: CanvasRenderingContext2D, w: number, h: number) {
  const particleColor = props.isDark
    ? 'rgba(99,179,237,0.7)' : 'rgba(49,130,206,0.6)'
  const lineBaseAlpha = props.isDark ? 0.2 : 0.15
  const lineRgb = props.isDark ? '99,179,237' : '49,130,206'

  ctx.clearRect(0, 0, w, h)

  for (const p of particles) {
    // mouse attraction
    const dx = mouse.x - p.x
    const dy = mouse.y - p.y
    const dist = Math.sqrt(dx * dx + dy * dy)
    if (dist < 150) {
      p.vx += (dx / dist) * 0.02
      p.vy += (dy / dist) * 0.02
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

  // connection lines
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
        ctx.strokeStyle = `rgba(${lineRgb},${alpha.toFixed(3)})`
        ctx.lineWidth = 1
        ctx.stroke()
      }
    }
  }

  animId = requestAnimationFrame(() => tick(ctx, w, h))
}
```

- [ ] **Step 4: Commit particle logic**

```bash
git add web/src/components/ParticleCanvas.vue
git commit -m "feat: add particle system logic to ParticleCanvas"
```

---

### Task 3: Wire up lifecycle and resize handling

**Files:**
- Modify: `web/src/components/ParticleCanvas.vue`

- [ ] **Step 1: Add onMounted / onUnmounted / resize logic**

```typescript
function startAnimation() {
  const canvas = canvasRef.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  const w = canvas.offsetWidth
  const h = canvas.offsetHeight
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
```

- [ ] **Step 2: Watch isDark prop to restart animation with new colors**

```typescript
watch(() => props.isDark, () => {
  cancelAnimationFrame(animId)
  startAnimation()
})
```

- [ ] **Step 3: Commit lifecycle wiring**

```bash
git add web/src/components/ParticleCanvas.vue
git commit -m "feat: wire lifecycle and resize to ParticleCanvas"
```

---

### Task 4: Integrate into LoginView.vue

**Files:**
- Modify: `web/src/views/LoginView.vue`

- [ ] **Step 1: Add import and inject in `<script setup>`**

Add after existing imports:

```typescript
import ParticleCanvas from '../components/ParticleCanvas.vue'
import { inject } from 'vue'

const isDark = inject<() => boolean>('isDark', () => false)
```

- [ ] **Step 2: Add ParticleCanvas to template**

Replace:
```html
<div class="login-page">
```
With:
```html
<div class="login-page">
  <ParticleCanvas :isDark="isDark()" class="particle-bg" />
```

- [ ] **Step 3: Update CSS — add particle-bg rule and make login-page position:relative, login-card position:relative z-index:1**

Add to `<style scoped>`:
```css
.login-page {
  position: relative;
}
.particle-bg {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  z-index: 0;
}
.login-card {
  position: relative;
  z-index: 1;
}
```

Note: `.login-page` already has `min-height: 100vh` — just add `position: relative` to the existing rule.

- [ ] **Step 4: Commit integration**

```bash
git add web/src/views/LoginView.vue
git commit -m "feat: integrate ParticleCanvas into LoginView"
```

---

### Task 5: Build and visual verification

**Files:** none (verification only)

- [ ] **Step 1: Build frontend**

```bash
cd web && npm run build
```

Expected: build succeeds with no errors.

- [ ] **Step 2: Start server**

```bash
go run ./cmd/spider serve --addr :8002 --data-dir ~/.spider/data
```

- [ ] **Step 3: Open login page in Playwright and verify**

```bash
# In a separate terminal
npx playwright open http://localhost:8002/login
```

Verify:
- Particles visible and moving on login page background
- Login card renders above particles (not obscured)
- Mouse movement causes nearby particles to drift toward cursor

- [ ] **Step 4: Toggle theme and verify color change**

Click the theme toggle button (☀️/🌙) — particles should shift from bright blue (dark) to deeper blue (light).

- [ ] **Step 5: Resize browser window**

Drag browser window narrower/wider — canvas should resize and particles redistribute without freezing.

- [ ] **Step 6: Final commit if any fixes were needed**

```bash
git add -p
git commit -m "fix: particle effect visual corrections"
```
