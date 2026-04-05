<template>
  <div class="app">
    <header class="nav">
      <div class="nav-brand">🕷 Spider</div>
      <nav class="nav-links">
        <RouterLink to="/hosts" class="nav-item">主机管理</RouterLink>
        <RouterLink to="/exec" class="nav-item">命令执行</RouterLink>
        <RouterLink to="/audit" class="nav-item">审计</RouterLink>
        <RouterLink to="/settings" class="nav-item">设置</RouterLink>
      </nav>
    </header>
    <main class="main">
      <RouterView />
    </main>
  </div>
</template>

<style>
* { box-sizing: border-box; margin: 0; padding: 0; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: #f5f5f5; color: #333; }
.app { min-height: 100vh; display: flex; flex-direction: column; }
.nav { background: #1a1a2e; color: #fff; display: flex; align-items: center; padding: 0 24px; height: 56px; gap: 32px; }
.nav-brand { font-size: 18px; font-weight: 600; letter-spacing: 0.5px; }
.nav-links { display: flex; gap: 4px; }
.nav-item { color: rgba(255,255,255,0.7); text-decoration: none; padding: 8px 16px; border-radius: 6px; font-size: 14px; transition: all 0.15s; }
.nav-item:hover { color: #fff; background: rgba(255,255,255,0.1); }
.nav-item.router-link-active { color: #fff; background: rgba(255,255,255,0.15); }
.main { flex: 1; padding: 24px; max-width: 1200px; width: 100%; margin: 0 auto; }

/* 通用组件 */
.page-header { display:flex; justify-content:space-between; align-items:center; margin-bottom:20px; }
.page-header h2 { font-size:20px; font-weight:600; }
.toolbar { display:flex; gap:12px; align-items:center; margin-bottom:16px; flex-wrap:wrap; }
.input { border:1px solid #ddd; border-radius:6px; padding:7px 10px; font-size:14px; outline:none; width:100%; }
.input:focus { border-color:#4f46e5; }
textarea.input { resize:vertical; font-family:inherit; }
.btn { border:1px solid #ddd; background:#fff; border-radius:6px; padding:7px 14px; font-size:14px; cursor:pointer; transition:all 0.15s; }
.btn:hover { background:#f5f5f5; }
.btn:disabled { opacity:0.5; cursor:not-allowed; }
.btn-primary { background:#4f46e5; color:#fff; border-color:#4f46e5; }
.btn-primary:hover { background:#4338ca; }
.btn-danger { color:#dc2626; border-color:#fca5a5; }
.btn-danger:hover { background:#fef2f2; }
.btn-sm { padding:4px 10px; font-size:13px; }
.table { width:100%; border-collapse:collapse; background:#fff; border-radius:8px; overflow:hidden; box-shadow:0 1px 3px rgba(0,0,0,0.08); }
.table th { background:#f9fafb; font-size:13px; font-weight:600; color:#6b7280; padding:10px 14px; text-align:left; border-bottom:1px solid #e5e7eb; }
.table td { padding:10px 14px; font-size:14px; border-bottom:1px solid #f3f4f6; }
.table tr:last-child td { border-bottom:none; }
.actions { display:flex; gap:6px; }
.badge { background:#e0e7ff; color:#4f46e5; border-radius:4px; padding:2px 8px; font-size:12px; }
.tag { background:#f3f4f6; border-radius:20px; padding:4px 12px; font-size:13px; cursor:pointer; border:1px solid transparent; }
.tag.active { background:#e0e7ff; color:#4f46e5; border-color:#c7d2fe; }
.tag.small { padding:2px 8px; font-size:12px; margin-right:4px; }
.tags { display:flex; gap:6px; flex-wrap:wrap; }
.modal-overlay { position:fixed; inset:0; background:rgba(0,0,0,0.4); display:flex; align-items:center; justify-content:center; z-index:100; }
.modal { background:#fff; border-radius:12px; padding:24px; width:480px; max-width:95vw; max-height:90vh; overflow-y:auto; }
.modal.wide { width:680px; }
.modal h3 { font-size:16px; font-weight:600; margin-bottom:20px; }
.modal-footer { display:flex; justify-content:flex-end; gap:10px; margin-top:20px; }
.form-row { display:flex; flex-direction:column; gap:6px; margin-bottom:14px; }
.form-row label { font-size:13px; font-weight:500; color:#374151; }
.section-title { font-size:13px; font-weight:600; color:#6b7280; margin-bottom:8px; text-transform:uppercase; letter-spacing:0.5px; }
.bulk-bar { margin-top:12px; display:flex; gap:12px; align-items:center; font-size:14px; }
.output { background:#1e1e2e; color:#cdd6f4; padding:12px; border-radius:6px; font-size:13px; overflow-x:auto; white-space:pre-wrap; word-break:break-all; margin-top:8px; }
.output.stderr { color:#f38ba8; }
.ok { color:#16a34a; font-size:14px; }
.err { color:#dc2626; font-size:14px; }
.dim { color:#9ca3af; font-size:13px; }
.code { font-family: 'SF Mono', Consolas, monospace; font-size:13px; }
.truncate { max-width:200px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.exec-layout { display:grid; grid-template-columns:240px 1fr; gap:20px; }
.exec-left { background:#fff; border-radius:8px; padding:16px; box-shadow:0 1px 3px rgba(0,0,0,0.08); }
.exec-right { background:#fff; border-radius:8px; padding:16px; box-shadow:0 1px 3px rgba(0,0,0,0.08); }
.host-list { display:flex; flex-direction:column; gap:6px; max-height:300px; overflow-y:auto; }
.host-item { display:flex; align-items:center; gap:8px; padding:6px; border-radius:6px; cursor:pointer; font-size:14px; }
.host-item:hover { background:#f9fafb; }
.host-item small { color:#9ca3af; font-size:12px; }
.results { margin-top:20px; display:flex; flex-direction:column; gap:12px; }
.result-block { border:1px solid #e5e7eb; border-radius:8px; overflow:hidden; }
.result-header { display:flex; gap:16px; align-items:center; padding:10px 14px; background:#f9fafb; font-size:14px; font-weight:500; }
.settings-card { background:#fff; border-radius:8px; padding:20px; box-shadow:0 1px 3px rgba(0,0,0,0.08); margin-bottom:20px; }
.settings-card h3 { font-size:15px; font-weight:600; margin-bottom:16px; }
.detail-meta { display:flex; flex-wrap:wrap; gap:16px; font-size:14px; margin-bottom:16px; color:#374151; }
.pagination { display:flex; gap:12px; align-items:center; margin-top:16px; justify-content:center; }
</style>
