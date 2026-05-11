# Login Page Particle Effect Design

## Overview

Add AI-themed particle animation to the login page background. Floating nodes with dynamic connections create a neural network visual that responds to mouse movement.

## Requirements

- **Visual style**: Floating particles with distance-based connecting lines (neural network aesthetic)
- **Interaction**: Light mouse interaction — particles gently attracted to cursor within range
- **Theme integration**: Particle colors adapt to dark/light theme automatically
- **Performance**: Smooth 60fps on desktop, no blocking of login form interaction

## Architecture

### Component Structure

**New component**: `web/src/components/ParticleCanvas.vue`

- Renders a `<canvas>` element that fills its container
- Accepts `isDark: boolean` prop to control color scheme
- Manages animation loop with `requestAnimationFrame`
- Tracks mouse position via `mousemove` listener

**Integration**: `LoginView.vue`

- Import `ParticleCanvas` component
- Inject `isDark` from App.vue via `inject('isDark')`
- Position canvas absolutely behind login card (z-index layering)

### Particle System

**Particle count**: 60 particles

**Particle properties**:
- Position (x, y)
- Velocity (vx, vy) — random initial values, magnitude ~0.3-0.8 px/frame
- Radius: 2px

**Behavior per frame**:
1. Update position: `x += vx`, `y += vy`
2. Bounce at canvas edges (reverse velocity component)
3. Mouse attraction: if distance to mouse < 150px, apply gentle force toward mouse (0.02 strength)
4. Draw particle as filled circle

**Connection lines**:
- For each particle pair, if distance < 120px, draw line between them
- Line opacity inversely proportional to distance: `opacity = (1 - distance/120) * baseOpacity`
- Base opacity depends on theme (see Colors section)

### Colors

**Dark theme** (`isDark = true`):
- Particle fill: `rgba(99, 179, 237, 0.7)` — bright blue
- Connection line: `rgba(99, 179, 237, 0.2)` — translucent blue

**Light theme** (`isDark = false`):
- Particle fill: `rgba(49, 130, 206, 0.6)` — darker blue
- Connection line: `rgba(49, 130, 206, 0.15)` — subtle blue

### Layout Integration

**LoginView.vue structure**:
```
<div class="login-page">
  <ParticleCanvas :isDark="isDark()" class="particle-bg" />
  <div class="login-card">...</div>
</div>
```

**CSS**:
```css
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

## Implementation Details

### Canvas Sizing

- Set canvas width/height to match container dimensions on mount
- Listen to `window.resize` and update canvas dimensions + reinitialize particles

### Animation Loop

- Use `requestAnimationFrame` for smooth 60fps
- Start loop on `onMounted`, cancel on `onUnmounted` to prevent memory leaks

### Mouse Tracking

- Add `mousemove` listener to canvas element
- Convert event coordinates to canvas-relative coordinates
- Store in reactive ref for use in particle update logic

### Performance Considerations

- 60 particles × 60 particles = 3600 distance checks per frame (acceptable for modern browsers)
- No need for spatial partitioning at this scale
- Canvas 2D API is hardware-accelerated on modern browsers

## Testing Strategy

1. **Visual verification**: Start dev server, navigate to login page, verify particles render and move
2. **Theme switching**: Toggle theme in App.vue, verify particle colors update
3. **Mouse interaction**: Move mouse over particles, verify attraction effect
4. **Resize handling**: Resize browser window, verify canvas resizes and particles redistribute
5. **Performance**: Open DevTools Performance tab, verify 60fps with no dropped frames

## Files Changed

- **New**: `web/src/components/ParticleCanvas.vue` (~120 lines)
- **Modified**: `web/src/views/LoginView.vue` (import component, add to template, update styles)

## Non-Goals

- Mobile optimization (login page primarily desktop-focused)
- Configurable particle count/colors (hardcoded for simplicity)
- Particle collision physics (unnecessary complexity)
- WebGL rendering (Canvas 2D sufficient for this scale)
