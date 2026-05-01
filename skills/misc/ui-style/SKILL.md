---
name: ui-style
description: 为 web 前端编写或修改页面布局和样式时使用。涵盖整体页面结构、视图布局 pattern、主题系统用法、卡片/面板规范、对比度要求、间距原则。触发词：优化、布局、样式、UI、卡片、颜色、主题、对比度、呼吸感、视图、页面。
---

# UI 布局与样式规范

## 整体页面结构

```
App.tsx (flex column, minHeight: 100vh, background: c.bgGradient)
├── TopNav (height: 52px, position: sticky, top: 0, z-index: 100)
└── main (flex: 1)
    └── <当前 View>
```

**TopNav** 布局：`display: flex; alignItems: stretch; height: 52px`
- 左：Logo 区（带右边框）
- 中：导航项（`flex: 1`），活跃项用底部边框 `borderBottom: 2px solid c.primary` 标识
- 右：主题切换按钮

**视图切换**（App.tsx state）：

| state | 渲染组件 |
|-------|---------|
| `'objects'` | CredentialsView |
| `'templates'` | TemplatesView |
| `'records'` | RecordsView（内嵌 RunView） |
| `'new-run'` | NewRunView |

---

## 布局 Pattern

### Pattern A：左右分栏（列表 + 详情）

用于 RecordsView、CredentialsView、TemplatesView。

```tsx
// 外层容器
<div style={{
  display: 'flex',
  height: 'calc(100vh - 52px)',
  background: c.bgGradient,
  overflow: 'hidden',
}}>
  {/* 左侧列表面板 */}
  <div style={{
    width: '26%',
    minWidth: 320,
    maxWidth: 400,
    background: c.panel,
    borderRight: `1px solid ${c.border}`,
    display: 'flex',
    flexDirection: 'column',
    flexShrink: 0,
  }}>
    {/* 工具栏 */}
    <div style={{ padding: '16px 16px 12px', borderBottom: `1px solid ${c.border}`,
      display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
      ...
    </div>
    {/* 列表区（可滚动） */}
    <div style={{ overflowY: 'auto', flex: 1 }}>
      {/* 列表项 */}
      <div style={{
        padding: '12px 16px',
        borderBottom: `1px solid ${c.border}`,
        borderLeft: selected ? '3px solid #6366f1' : '3px solid transparent',
        background: selected ? 'rgba(99,102,241,0.15)' : hovered ? c.rowHover : 'transparent',
        cursor: 'pointer',
      }}>
        ...
      </div>
    </div>
  </div>

  {/* 右侧详情区 */}
  <div style={{ flex: 1, overflow: 'hidden', minWidth: 0 }}>
    {selected ? <DetailView /> : <EmptyState />}
  </div>
</div>
```

**空状态**（右侧无选中时）：
```tsx
<div style={{ height: '100%', display: 'flex', flexDirection: 'column',
  alignItems: 'center', justifyContent: 'center', gap: 12 }}>
  <div style={{ color: c.border, fontSize: 40 }}>←</div>
  <div style={{ color: c.muted, fontSize: 14 }}>选择左侧记录查看详情</div>
</div>
```

---

### Pattern B：全屏纵向（进度条 + 滚动主体）

用于 RunView（嵌入 RecordsView 右侧）。

```tsx
<div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
  {/* 固定顶部工具条 */}
  <div style={{ background: c.panel, padding: '8px 16px',
    borderBottom: `1px solid ${c.border}`,
    display: 'flex', alignItems: 'center', gap: 12, flexShrink: 0 }}>
    ...
  </div>

  {/* 可滚动主体 */}
  <div style={{ flex: 1, overflowY: 'auto', padding: 16,
    display: 'flex', flexDirection: 'column', gap: 12 }}>
    {/* 指标卡片行 */}
    <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}>
      <MetricCard ... />
    </div>
    {/* 2 列图表 */}
    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
      <ChartCard title="吞吐量趋势">...</ChartCard>
      <ChartCard title="延迟分布">...</ChartCard>
    </div>
    {/* 全宽图表 */}
    <ChartCard title="请求趋势">...</ChartCard>
  </div>
</div>
```

---

### Pattern C：单列表单（居中，限宽）

用于 NewRunView（maxWidth: 800px）、ConfigView（maxWidth: 680px）。

```tsx
<div style={{ minHeight: 'calc(100vh - 52px)', background: c.bgGradient }}>
  <div style={{ maxWidth: 800, padding: '32px 40px', boxSizing: 'border-box' }}>
    {/* 页头：返回 + 标题 */}
    <div style={{ display: 'flex', alignItems: 'center', gap: 16, marginBottom: 24 }}>
      <button ...>← 返回</button>
      <div>
        <h2 style={{ color: c.text, fontSize: 20, fontWeight: 700, margin: 0 }}>标题</h2>
        <p style={{ color: c.muted, fontSize: 12, margin: 0 }}>副标题</p>
      </div>
    </div>

    {/* 表单区块（FormBlock） */}
    <div style={{ background: c.surface, border: `1px solid ${c.border}`,
      borderRadius: 12, padding: '20px 24px', boxShadow: c.cardShadow, marginBottom: 16 }}>
      {/* 3 列参数网格 */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 12 }}>
        ...
      </div>
    </div>

    {/* 提交行 */}
    <div style={{ display: 'flex', gap: 12, alignItems: 'center', paddingBottom: 48 }}>
      <input style={{ flex: 1, ...inp }} />
      <button style={{ background: c.primary, ... }}>▶ 开始</button>
    </div>
  </div>
</div>
```

**常用内部网格**：

| 场景 | gridTemplateColumns |
|------|---------------------|
| 3 列数字参数 | `1fr 1fr 1fr` |
| 2 列字段 | `1fr 1fr` |
| 主字段 + 操作按钮 + 次字段 | `flex: 1.5` + `flexShrink: 0` + `flex: 1` |

---

## 卡片规范

### MetricCard（数据指标卡）

```tsx
<div style={{
  background: c.cardBg,
  border: `1px solid ${c.border}`,
  borderRadius: 10,
  padding: '14px 20px',
  textAlign: 'center',
  minWidth: 96,
  boxShadow: c.cardShadow,
}}>
  <div style={{ color: c.muted, fontSize: 11, fontWeight: 600,
    textTransform: 'uppercase', letterSpacing: '0.07em', marginBottom: 6 }}>
    {label}
  </div>
  <div style={{ color: c.text, fontSize: 24, fontWeight: 700,
    letterSpacing: '-0.02em', lineHeight: 1 }}>
    {value}
  </div>
</div>
```

### ChartCard（图表容器卡）

```tsx
<div style={{
  background: c.cardBg,
  border: `1px solid ${c.border}`,
  borderRadius: 10,
  padding: '14px 16px',
  boxShadow: c.cardShadow,
}}>
  <div style={{ color: c.muted, fontSize: 11, fontWeight: 600,
    marginBottom: 8, letterSpacing: '0.07em', textTransform: 'uppercase' }}>
    {title}
  </div>
  {children}
</div>
```

### FormBlock（表单区块）

```tsx
<div style={{
  background: c.surface,
  border: `1px solid ${c.border}`,
  borderRadius: 12,
  padding: '20px 24px',
  backdropFilter: 'blur(10px)',
  boxShadow: c.cardShadow,
  marginBottom: 16,
}}>
```

---

## 间距原则

| 位置 | 值 |
|------|----|
| 卡片之间 gap | `10–12px` |
| 数据卡 padding | `14px 20px` |
| 图表卡 padding | `14px 16px` |
| 表单块 padding | `20px 24px` |
| 页面主体 padding | `16px` |
| 标签 → 数值 marginBottom | `6px` |
| 区块标题 marginBottom | `8px` |
| 表单区块 marginBottom | `16px` |
| 页头 marginBottom | `24px` |

---

## 主题系统

所有颜色必须通过 `useTheme()` 获取，**禁止硬编码颜色值**。

```tsx
import { useTheme } from '../theme'

function MyComponent() {
  const { tokens: c, theme } = useTheme()
}
```

### Token 列表

| Token | 暗色值 | 亮色值 | 用途 |
|-------|--------|--------|------|
| `c.bg` | `#0d0f1a` | `#f0f2f8` | 页面背景 |
| `c.bgGradient` | 带 indigo/purple 渐变 | 带 indigo 渐变 | 主视图容器背景 |
| `c.nav` | `#080a12` | `#ffffff` | 顶部导航栏背景 |
| `c.navBorder` | `#1e2338` | `#e2e4ed` | 导航栏底部边框 |
| `c.surface` | `rgba(30,33,50,0.92)` | `#ffffff` | 表单块、弹层等主要卡片面 |
| `c.cardBg` | `rgba(30,33,50,0.92)` | `#ffffff` | MetricCard / ChartCard 数据卡片背景 |
| `c.panel` | `#12141f` | `#ffffff` | 侧边栏、底部面板背景 |
| `c.border` | `#2c3150` | `#d8dce8` | 通用边框、分割线 |
| `c.borderFocus` | `#6366f1` | `#6366f1` | 输入框聚焦边框 |
| `c.primary` | `#6366f1` | `#6366f1` | 主色（indigo）按钮、选中态 |
| `c.primaryHover` | `#4f52d4` | `#4f52d4` | 主色悬停 |
| `c.accent` | `#e94560` | `#e94560` | 强调色（红）警告/错误高亮 |
| `c.text` | `#eceef5` | `#111827` | 主要正文 |
| `c.textSub` | `#b0b8c8` | `#374151` | 次要正文（对比度 ≥ 8:1） |
| `c.muted` | `#b0b8c8` | `#4b5563` | 标签、辅助说明（对比度 ≥ 7:1） |
| `c.label` | `#8892a4` | `#4b5563` | 时间戳等最低优先级文字 |
| `c.green` | `#4ade80` | `#15803d` | 成功/运行中 |
| `c.red` | `#f87171` | `#dc2626` | 错误/失败 |
| `c.yellow` | `#fbbf24` | `#b45309` | 警告 |
| `c.purple` | `#a78bfa` | `#6d28d9` | 标签/徽章 |
| `c.inputBg` | `rgba(15,17,28,0.6)` | `#ffffff` | 输入框背景 |
| `c.rowAlt` | `rgba(255,255,255,0.018)` | `rgba(0,0,0,0.018)` | 表格奇偶行交替背景 |
| `c.rowHover` | `rgba(99,102,241,0.07)` | `rgba(99,102,241,0.05)` | 表格行悬停背景 |
| `c.cardShadow` | `0 1px 3px rgba(0,0,0,0.5), 0 4px 20px rgba(0,0,0,0.3)` | `0 1px 3px rgba(0,0,0,0.06), 0 4px 16px rgba(0,0,0,0.07)` | 卡片阴影（双层） |

---

## 对比度要求（WCAG AAA ≥ 7:1）

| Token | 暗色值 | 暗色对比度 | 亮色值 | 亮色对比度 |
|-------|--------|-----------|--------|-----------|
| `text` | `#eceef5` | ~14:1 ✓ | `#111827` | ~18:1 ✓ |
| `textSub` | `#b0b8c8` | ~8.6:1 ✓ | `#374151` | ~10:1 ✓ |
| `muted` | `#b0b8c8` | ~8.6:1 ✓ | `#4b5563` | ~7:1 ✓ |
| `label` | `#8892a4` | ~5.5:1 ⚠ | `#4b5563` | ~7:1 ✓ |

`label` 仅用于时间戳等非关键文字，暗色下低于 7:1 可接受。

---

## 状态色徽章模式

rgba 透明度在两套主题下通用；`color` 字段用 token，亮色下自动切换为更深色值。

```tsx
// 成功/运行中
{ bg: 'rgba(74,222,128,0.12)', color: c.green, border: 'rgba(74,222,128,0.3)' }
// 主色/已完成
{ bg: 'rgba(99,102,241,0.12)', color: c.primary, border: 'rgba(99,102,241,0.3)' }
// 错误/已中止
{ bg: 'rgba(248,113,113,0.12)', color: c.red, border: 'rgba(248,113,113,0.3)' }
// 警告
{ bg: 'rgba(251,191,36,0.1)', color: c.yellow, border: 'rgba(251,191,36,0.3)' }
// 紫色标签
{ bg: 'rgba(167,139,250,0.1)', color: c.purple, border: 'rgba(167,139,250,0.25)' }
```

徽章通用样式：`fontSize: 11, fontWeight: 600, padding: '2px 8px', borderRadius: 4`

---

## 输入框样式

```tsx
const inp: React.CSSProperties = {
  background: c.inputBg,
  border: `1px solid ${c.border}`,
  borderRadius: 8,
  height: 44,
  padding: '0 12px',
  color: c.text,
  fontSize: 14,
  width: '100%',
  boxSizing: 'border-box',
  outline: 'none',
}
```

## 标签（Label）样式

```tsx
const lbl: React.CSSProperties = {
  color: c.muted,
  fontSize: 11,
  fontWeight: 600,
  textTransform: 'uppercase',
  letterSpacing: '0.08em',
  marginBottom: 6,
  display: 'block',
}
```

---

## 禁止事项

- 禁止使用 `#0f3460`、`#16213e`、`#7ec8e3` 等硬编码颜色
- 禁止在组件内直接写死 `background: '#xxx'`，必须用 `c.cardBg` / `c.surface` 等 token
- 新增颜色语义时，先在 `web/src/theme.tsx` 的 `dark` 和 `light` 对象中同时添加 token，再使用
- 不要为 `label` 级别的文字（时间戳、次要说明）强求 7:1，但 `muted` 及以上必须满足


所有颜色必须通过 `useTheme()` 获取，**禁止硬编码颜色值**。

```tsx
import { useTheme } from '../theme'

function MyComponent() {
  const { tokens: c, theme } = useTheme()
  // 使用 c.text, c.surface, c.border 等
}
```

### 完整 Token 列表

| Token | 暗色值 | 亮色值 | 用途 |
|-------|--------|--------|------|
| `c.bg` | `#0d0f1a` | `#f0f2f8` | 页面背景 |
| `c.bgGradient` | 带 indigo/purple 渐变 | 带 indigo 渐变 | 主视图容器背景 |
| `c.nav` | `#080a12` | `#ffffff` | 顶部导航栏背景 |
| `c.navBorder` | `#1e2338` | `#e2e4ed` | 导航栏底部边框 |
| `c.surface` | `rgba(30,33,50,0.92)` | `#ffffff` | 表单块、弹层等主要卡片面 |
| `c.cardBg` | `rgba(30,33,50,0.92)` | `#ffffff` | MetricCard / ChartCard 数据卡片背景 |
| `c.panel` | `#12141f` | `#ffffff` | 侧边栏、底部面板背景 |
| `c.border` | `#2c3150` | `#d8dce8` | 通用边框、分割线 |
| `c.borderFocus` | `#6366f1` | `#6366f1` | 输入框聚焦边框 |
| `c.primary` | `#6366f1` | `#6366f1` | 主色（indigo）按钮、选中态 |
| `c.primaryHover` | `#4f52d4` | `#4f52d4` | 主色悬停 |
| `c.accent` | `#e94560` | `#e94560` | 强调色（红）警告/错误高亮 |
| `c.text` | `#eceef5` | `#111827` | 主要正文 |
| `c.textSub` | `#b0b8c8` | `#374151` | 次要正文（对比度 ≥ 8:1） |
| `c.muted` | `#b0b8c8` | `#4b5563` | 标签、辅助说明（对比度 ≥ 7:1） |
| `c.label` | `#8892a4` | `#4b5563` | 时间戳等最低优先级文字 |
| `c.green` | `#4ade80` | `#15803d` | 成功/运行中 |
| `c.red` | `#f87171` | `#dc2626` | 错误/失败 |
| `c.yellow` | `#fbbf24` | `#b45309` | 警告 |
| `c.purple` | `#a78bfa` | `#6d28d9` | 标签/徽章 |
| `c.inputBg` | `rgba(15,17,28,0.6)` | `#ffffff` | 输入框背景 |
| `c.rowAlt` | `rgba(255,255,255,0.018)` | `rgba(0,0,0,0.018)` | 表格奇偶行交替背景 |
| `c.rowHover` | `rgba(99,102,241,0.07)` | `rgba(99,102,241,0.05)` | 表格行悬停背景 |
| `c.cardShadow` | `0 1px 3px rgba(0,0,0,0.5), 0 4px 20px rgba(0,0,0,0.3)` | `0 1px 3px rgba(0,0,0,0.06), 0 4px 16px rgba(0,0,0,0.07)` | 卡片阴影（双层） |

---

## 卡片规范

### MetricCard（数据指标卡）

```tsx
<div style={{
  background: c.cardBg,
  border: `1px solid ${c.border}`,
  borderRadius: 10,
  padding: '14px 20px',
  textAlign: 'center',
  minWidth: 96,
  boxShadow: c.cardShadow,
}}>
  <div style={{ color: c.muted, fontSize: 11, fontWeight: 600,
    textTransform: 'uppercase', letterSpacing: '0.07em', marginBottom: 6 }}>
    {label}
  </div>
  <div style={{ color: c.text, fontSize: 24, fontWeight: 700,
    letterSpacing: '-0.02em', lineHeight: 1 }}>
    {value}
  </div>
</div>
```

### ChartCard（图表容器卡）

```tsx
<div style={{
  background: c.cardBg,
  border: `1px solid ${c.border}`,
  borderRadius: 10,
  padding: '14px 16px',
  boxShadow: c.cardShadow,
}}>
  <div style={{ color: c.muted, fontSize: 11, fontWeight: 600,
    marginBottom: 8, letterSpacing: '0.07em', textTransform: 'uppercase' }}>
    {title}
  </div>
  {children}
</div>
```

### FormBlock（表单区块）

```tsx
<div style={{
  background: c.surface,
  border: `1px solid ${c.border}`,
  borderRadius: 12,
  padding: '20px 24px',
  backdropFilter: 'blur(10px)',
  boxShadow: c.cardShadow,
  marginBottom: 16,
}}>
```

---

## 间距与呼吸感原则

- 卡片之间 gap：`10–12px`
- 卡片内 padding：`14px 20px`（数据卡）/ `14px 16px`（图表卡）/ `20px 24px`（表单块）
- 页面主体 padding：`16px`
- 标签与数值之间 marginBottom：`6px`
- 区块标题 marginBottom：`8px`

---

## 对比度要求（WCAG AAA ≥ 7:1）

| Token | 暗色值 | 暗色对比度 | 亮色值 | 亮色对比度 |
|-------|--------|-----------|--------|-----------|
| `text` | `#eceef5` | ~14:1 ✓ | `#111827` | ~18:1 ✓ |
| `textSub` | `#b0b8c8` | ~8.6:1 ✓ | `#374151` | ~10:1 ✓ |
| `muted` | `#b0b8c8` | ~8.6:1 ✓ | `#4b5563` | ~7:1 ✓ |
| `label` | `#8892a4` | ~5.5:1 ⚠ | `#4b5563` | ~7:1 ✓ |

`label` 仅用于时间戳等非关键文字，暗色下低于 7:1 可接受。

---

## 状态色徽章模式

rgba 透明度在两套主题下通用（亮色背景更白，alpha 叠加后仍可见）：

```tsx
// 成功/运行中
{ bg: 'rgba(74,222,128,0.12)', color: c.green, border: 'rgba(74,222,128,0.3)' }

// 主色/已完成
{ bg: 'rgba(99,102,241,0.12)', color: c.primary, border: 'rgba(99,102,241,0.3)' }

// 错误/已中止
{ bg: 'rgba(248,113,113,0.12)', color: c.red, border: 'rgba(248,113,113,0.3)' }

// 警告
{ bg: 'rgba(251,191,36,0.1)', color: c.yellow, border: 'rgba(251,191,36,0.3)' }

// 紫色标签
{ bg: 'rgba(167,139,250,0.1)', color: c.purple, border: 'rgba(167,139,250,0.25)' }
```

> `color` 字段使用 `c.green` / `c.red` 等 token，亮色主题下这些 token 会自动切换为更深的色值（如 `#15803d`），确保在白色背景上的对比度。

徽章通用样式：
```tsx
{ fontSize: 11, fontWeight: 600, padding: '2px 8px', borderRadius: 4,
  border: `1px solid ${borderColor}` }
```

---

## 输入框样式

```tsx
const inp: React.CSSProperties = {
  background: c.inputBg,
  border: `1px solid ${c.border}`,
  borderRadius: 8,
  height: 44,
  padding: '0 12px',
  color: c.text,
  fontSize: 14,
  width: '100%',
  boxSizing: 'border-box',
  outline: 'none',
}
```

---

## 标签（Label）样式

```tsx
const lbl: React.CSSProperties = {
  color: c.muted,
  fontSize: 11,
  fontWeight: 600,
  textTransform: 'uppercase',
  letterSpacing: '0.08em',
  marginBottom: 6,
  display: 'block',
}
```

---

## 禁止事项

- 禁止使用 `#0f3460`、`#16213e`、`#7ec8e3` 等硬编码颜色
- 禁止在组件内直接写死 `background: '#xxx'`，必须用 `c.cardBg` / `c.surface` 等 token
- 新增颜色语义时，先在 `web/src/theme.tsx` 的 `dark` 和 `light` 对象中同时添加 token，再使用
- 不要为 `label` 级别的文字（时间戳、次要说明）强求 7:1，但 `muted` 及以上必须满足
