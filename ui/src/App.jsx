import React, { useState, useEffect, useCallback, useRef } from 'react'
import {
  PieChart, Pie, Cell, ResponsiveContainer, BarChart, Bar,
  XAxis, YAxis, Tooltip, RadialBarChart, RadialBar, Legend,
  AreaChart, Area, CartesianGrid, ReferenceLine,
} from 'recharts'

const API_BASE = import.meta.env.VITE_API_URL || ''

// ─── Color Tokens ────────────────────────────────────────────────────────────
const C = {
  bg:       '#0a0c0f',
  surface:  '#111318',
  border:   '#1e2028',
  muted:    '#3a3d47',
  text:     '#e2e4e9',
  dim:      '#8b8fa3',
  cyan:     '#00d4ff',
  amber:    '#f59e0b',
  red:      '#ef4444',
  green:    '#10b981',
  purple:   '#8b5cf6',
}

const RISK_COLORS = {
  CRITICAL: C.red,
  HIGH:     C.amber,
  MEDIUM:   C.purple,
  LOW:      C.green,
}

const FONT = {
  mono: "'JetBrains Mono', monospace",
  head: "'Syne', sans-serif",
}

// ─── Toast System ────────────────────────────────────────────────────────────
let _toastId = 0
function useToast() {
  const [toasts, setToasts] = useState([])
  const add = useCallback((msg, type = 'info') => {
    const id = ++_toastId
    setToasts(prev => [...prev, { id, msg, type }])
    setTimeout(() => setToasts(prev => prev.filter(t => t.id !== id)), 3500)
  }, [])
  return { toasts, add }
}

function ToastContainer({ toasts }) {
  return (
    <div style={{ position: 'fixed', top: 20, right: 20, zIndex: 9999, display: 'flex', flexDirection: 'column', gap: 8 }}>
      {toasts.map(t => (
        <div key={t.id} style={{
          background: t.type === 'error' ? C.red : t.type === 'success' ? C.green : C.cyan,
          color: '#000', padding: '10px 18px', borderRadius: 6, fontFamily: FONT.mono,
          fontSize: 13, fontWeight: 600, boxShadow: '0 4px 20px rgba(0,0,0,.5)',
          animation: 'slideIn .25s ease-out',
        }}>
          {t.msg}
        </div>
      ))}
    </div>
  )
}

// ─── Helpers ─────────────────────────────────────────────────────────────────
function fmt(n, decimals = 1) {
  if (n == null) return '—'
  if (typeof n === 'string') return n
  if (n >= 1e9) return (n / 1e9).toFixed(decimals) + 'G'
  if (n >= 1e6) return (n / 1e6).toFixed(decimals) + 'M'
  if (n >= 1e3) return (n / 1e3).toFixed(decimals) + 'K'
  return n.toFixed(decimals)
}

function fmtCPU(cores) {
  if (cores == null) return '—'
  if (cores >= 1) return cores.toFixed(2) + ' cores'
  return (cores * 1000).toFixed(0) + 'm'
}

function fmtMem(gib) {
  if (gib == null) return '—'
  if (gib >= 1) return gib.toFixed(2) + ' GiB'
  return (gib * 1024).toFixed(0) + ' MiB'
}

function fmtDollars(v) {
  if (v == null) return '—'
  return '$' + Number(v).toFixed(2)
}

function pct(v) {
  if (v == null) return '—'
  return (v * 100).toFixed(1) + '%'
}

async function apiFetch(path, options = {}) {
  const url = API_BASE + path
  const res = await fetch(url, {
    headers: { 'Content-Type': 'application/json', ...options.headers },
    ...options,
  })
  if (!res.ok) throw new Error(`API ${res.status}: ${res.statusText}`)
  return res.json()
}

function copyToClipboard(text) {
  if (navigator.clipboard) return navigator.clipboard.writeText(text)
  const ta = document.createElement('textarea')
  ta.value = text
  document.body.appendChild(ta)
  ta.select()
  document.execCommand('copy')
  document.body.removeChild(ta)
}

// ─── Shared UI Atoms ─────────────────────────────────────────────────────────
function Badge({ children, color }) {
  return (
    <span style={{
      display: 'inline-block', padding: '2px 10px', borderRadius: 4,
      background: color + '22', color, fontFamily: FONT.mono, fontSize: 11,
      fontWeight: 600, letterSpacing: '.5px', textTransform: 'uppercase',
    }}>
      {children}
    </span>
  )
}

function KPICard({ label, value, sub, accent = C.cyan }) {
  return (
    <div style={{
      background: C.surface, border: `1px solid ${C.border}`, borderRadius: 8,
      padding: '20px 24px', flex: '1 1 180px', minWidth: 160,
    }}>
      <div style={{ fontFamily: FONT.mono, fontSize: 11, color: C.dim, textTransform: 'uppercase', letterSpacing: 1, marginBottom: 8 }}>{label}</div>
      <div style={{ fontFamily: FONT.mono, fontSize: 28, fontWeight: 700, color: accent }}>{value}</div>
      {sub && <div style={{ fontFamily: FONT.mono, fontSize: 11, color: C.dim, marginTop: 4 }}>{sub}</div>}
    </div>
  )
}

function WasteBar({ value }) {
  const v = Math.min(Math.max(value || 0, 0), 100)
  const color = v > 70 ? C.red : v > 40 ? C.amber : C.green
  return (
    <div style={{ width: '100%', height: 8, background: C.border, borderRadius: 4, overflow: 'hidden' }}>
      <div style={{ width: v + '%', height: '100%', background: color, borderRadius: 4, transition: 'width .4s ease' }} />
    </div>
  )
}

function Skeleton({ width = '100%', height = 16 }) {
  return <div style={{ width, height, background: C.border, borderRadius: 4, animation: 'pulse 1.5s ease-in-out infinite' }} />
}

function LoadingView() {
  return (
    <div style={{ padding: 40, display: 'flex', flexDirection: 'column', gap: 16 }}>
      {[1, 2, 3, 4, 5].map(i => <Skeleton key={i} height={60} />)}
    </div>
  )
}

function ErrorView({ message, onRetry }) {
  return (
    <div style={{ padding: 60, textAlign: 'center' }}>
      <div style={{ fontSize: 48, marginBottom: 16 }}>⚠</div>
      <div style={{ fontFamily: FONT.head, fontSize: 20, color: C.red, marginBottom: 8 }}>Connection Error</div>
      <div style={{ fontFamily: FONT.mono, fontSize: 13, color: C.dim, marginBottom: 24 }}>{message}</div>
      {onRetry && <button onClick={onRetry} style={btnStyle()}>Retry</button>}
    </div>
  )
}

function btnStyle(accent = C.cyan) {
  return {
    background: 'transparent', border: `1px solid ${accent}`, color: accent,
    padding: '8px 20px', borderRadius: 6, fontFamily: FONT.mono, fontSize: 13,
    cursor: 'pointer', fontWeight: 600, letterSpacing: '.5px',
    transition: 'all .2s',
  }
}

function btnSolid(accent = C.cyan) {
  return {
    background: accent, border: 'none', color: '#000',
    padding: '10px 24px', borderRadius: 6, fontFamily: FONT.mono, fontSize: 13,
    cursor: 'pointer', fontWeight: 700, letterSpacing: '.5px',
  }
}

// ─── NAV ICONS (SVG inline) ─────────────────────────────────────────────────
const NAV_ITEMS = [
  { id: 'overview',   label: 'Overview',   icon: '◉' },
  { id: 'workloads',  label: 'Workloads',  icon: '☰' },
  { id: 'recommend',  label: 'Recs',       icon: '⚡' },
  { id: 'settings',   label: 'Settings',   icon: '⚙' },
]

// ═══════════════════════════════════════════════════════════════════════════════
// CLUSTER OVERVIEW VIEW
// ═══════════════════════════════════════════════════════════════════════════════
function ClusterOverview({ summary, workloads, namespaces, loading }) {
  if (loading) return <LoadingView />

  const totalWorkloads = summary?.total_workloads ?? workloads?.length ?? 0
  const totalContainers = summary?.total_containers ?? (workloads || []).reduce((s, w) => s + (w.containers?.length || 0), 0)
  const cpuWaste = summary?.cpu_waste_percent ?? 0
  const memWaste = summary?.mem_waste_percent ?? 0
  const estSavings = summary?.estimated_monthly_saving_usd ?? 0

  // Build ring chart data
  const cpuData = [
    { name: 'Waste', value: cpuWaste },
    { name: 'Used', value: 100 - cpuWaste },
  ]
  const memData = [
    { name: 'Waste', value: memWaste },
    { name: 'Used', value: 100 - memWaste },
  ]

  // Risk distribution
  const riskCounts = { CRITICAL: 0, HIGH: 0, MEDIUM: 0, LOW: 0 }
  ;(workloads || []).forEach(w => {
    const risk = w.overall_risk || 'LOW'
    if (riskCounts[risk] !== undefined) riskCounts[risk]++
  })
  const riskData = Object.entries(riskCounts).map(([name, value]) => ({ name, value }))

  // Namespace heatmap
  const nsMap = {}
  ;(workloads || []).forEach(w => {
    const ns = w.namespace || 'default'
    if (!nsMap[ns]) nsMap[ns] = { count: 0, waste: 0 }
    nsMap[ns].count++
    nsMap[ns].waste += (w.overall_waste_score || 0)
  })
  const nsHeat = Object.entries(nsMap).map(([name, d]) => ({
    name, count: d.count, avgWaste: d.count > 0 ? d.waste / d.count : 0,
  })).sort((a, b) => b.avgWaste - a.avgWaste)

  // Recent recommendations
  const recentRecs = (workloads || [])
    .filter(w => w.overall_risk === 'CRITICAL' || w.overall_risk === 'HIGH')
    .slice(0, 6)

  const RING_COLORS = [C.cyan, C.border]
  const RING_COLORS_MEM = [C.amber, C.border]

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
      {/* KPI Row */}
      <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap' }}>
        <KPICard label="Workloads" value={totalWorkloads} />
        <KPICard label="Containers" value={totalContainers} />
        <KPICard label="CPU Waste" value={pct(cpuWaste / 100)} accent={cpuWaste > 50 ? C.red : C.amber} />
        <KPICard label="Mem Waste" value={pct(memWaste / 100)} accent={memWaste > 50 ? C.red : C.amber} />
        <KPICard label="Est. Savings" value={fmtDollars(estSavings)} sub="/month" accent={C.green} />
      </div>

      {/* Charts Row */}
      <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap' }}>
        {/* CPU Ring */}
        <div style={{ flex: '1 1 280px', background: C.surface, border: `1px solid ${C.border}`, borderRadius: 8, padding: 20 }}>
          <div style={{ fontFamily: FONT.head, fontSize: 14, color: C.text, marginBottom: 12, fontWeight: 600 }}>CPU Utilization</div>
          <ResponsiveContainer width="100%" height={200}>
            <PieChart>
              <Pie data={cpuData} cx="50%" cy="50%" innerRadius={60} outerRadius={80} dataKey="value" startAngle={90} endAngle={-270}>
                {cpuData.map((_, i) => <Cell key={i} fill={RING_COLORS[i]} />)}
              </Pie>
              <Tooltip contentStyle={{ background: C.surface, border: `1px solid ${C.border}`, fontFamily: FONT.mono, fontSize: 12 }} />
            </PieChart>
          </ResponsiveContainer>
          <div style={{ textAlign: 'center', fontFamily: FONT.mono, fontSize: 13, color: C.dim }}>{cpuWaste.toFixed(1)}% wasted</div>
        </div>

        {/* Memory Ring */}
        <div style={{ flex: '1 1 280px', background: C.surface, border: `1px solid ${C.border}`, borderRadius: 8, padding: 20 }}>
          <div style={{ fontFamily: FONT.head, fontSize: 14, color: C.text, marginBottom: 12, fontWeight: 600 }}>Memory Utilization</div>
          <ResponsiveContainer width="100%" height={200}>
            <PieChart>
              <Pie data={memData} cx="50%" cy="50%" innerRadius={60} outerRadius={80} dataKey="value" startAngle={90} endAngle={-270}>
                {memData.map((_, i) => <Cell key={i} fill={RING_COLORS_MEM[i]} />)}
              </Pie>
              <Tooltip contentStyle={{ background: C.surface, border: `1px solid ${C.border}`, fontFamily: FONT.mono, fontSize: 12 }} />
            </PieChart>
          </ResponsiveContainer>
          <div style={{ textAlign: 'center', fontFamily: FONT.mono, fontSize: 13, color: C.dim }}>{memWaste.toFixed(1)}% wasted</div>
        </div>

        {/* Risk Distribution */}
        <div style={{ flex: '1 1 280px', background: C.surface, border: `1px solid ${C.border}`, borderRadius: 8, padding: 20 }}>
          <div style={{ fontFamily: FONT.head, fontSize: 14, color: C.text, marginBottom: 12, fontWeight: 600 }}>Risk Distribution</div>
          <ResponsiveContainer width="100%" height={200}>
            <PieChart>
              <Pie data={riskData} cx="50%" cy="50%" innerRadius={50} outerRadius={75} dataKey="value" paddingAngle={3}>
                {riskData.map((entry, i) => <Cell key={i} fill={RISK_COLORS[entry.name] || C.muted} />)}
              </Pie>
              <Tooltip contentStyle={{ background: C.surface, border: `1px solid ${C.border}`, fontFamily: FONT.mono, fontSize: 12 }} />
            </PieChart>
          </ResponsiveContainer>
          <div style={{ display: 'flex', justifyContent: 'center', gap: 12, flexWrap: 'wrap' }}>
            {riskData.map(r => (
              <div key={r.name} style={{ display: 'flex', alignItems: 'center', gap: 4, fontFamily: FONT.mono, fontSize: 11, color: C.dim }}>
                <span style={{ width: 8, height: 8, borderRadius: '50%', background: RISK_COLORS[r.name] }} />
                {r.name} ({r.value})
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Namespace Heatmap & Recent Recs */}
      <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap' }}>
        {/* Namespace Heatmap */}
        <div style={{ flex: '1 1 400px', background: C.surface, border: `1px solid ${C.border}`, borderRadius: 8, padding: 20 }}>
          <div style={{ fontFamily: FONT.head, fontSize: 14, color: C.text, marginBottom: 16, fontWeight: 600 }}>Namespace Heatmap</div>
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            {nsHeat.length === 0 && <div style={{ fontFamily: FONT.mono, fontSize: 12, color: C.dim }}>No data</div>}
            {nsHeat.map(ns => {
              const intensity = Math.min(ns.avgWaste / 100, 1)
              const bg = intensity > 0.6 ? C.red : intensity > 0.3 ? C.amber : C.green
              return (
                <div key={ns.name} style={{
                  padding: '8px 14px', borderRadius: 6, background: bg + '30',
                  border: `1px solid ${bg}55`, fontFamily: FONT.mono, fontSize: 11, color: C.text,
                }}>
                  <div style={{ fontWeight: 600 }}>{ns.name}</div>
                  <div style={{ color: C.dim, marginTop: 2 }}>{ns.count} workloads · {ns.avgWaste.toFixed(0)}% waste</div>
                </div>
              )
            })}
          </div>
        </div>

        {/* Recent Recommendations */}
        <div style={{ flex: '1 1 400px', background: C.surface, border: `1px solid ${C.border}`, borderRadius: 8, padding: 20 }}>
          <div style={{ fontFamily: FONT.head, fontSize: 14, color: C.text, marginBottom: 16, fontWeight: 600 }}>Recent Recommendations</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            {recentRecs.length === 0 && <div style={{ fontFamily: FONT.mono, fontSize: 12, color: C.dim }}>No critical recommendations</div>}
            {recentRecs.map((w, i) => (
              <div key={i} style={{
                display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                padding: '8px 12px', borderRadius: 6, background: C.bg, border: `1px solid ${C.border}`,
              }}>
                <div>
                  <span style={{ fontFamily: FONT.mono, fontSize: 12, color: C.text }}>{w.name}</span>
                  <span style={{ fontFamily: FONT.mono, fontSize: 11, color: C.dim, marginLeft: 8 }}>{w.namespace}</span>
                </div>
                <Badge color={RISK_COLORS[w.overall_risk]}>{w.overall_risk}</Badge>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}

// ═══════════════════════════════════════════════════════════════════════════════
// WORKLOAD EXPLORER VIEW
// ═══════════════════════════════════════════════════════════════════════════════
function WorkloadExplorer({ workloads, namespaces, loading, onSelect }) {
  const [nsFilter, setNsFilter] = useState('')
  const [riskFilter, setRiskFilter] = useState('')
  const [sortCol, setSortCol] = useState('overall_waste_score')
  const [sortDir, setSortDir] = useState('desc')
  const [search, setSearch] = useState('')

  if (loading) return <LoadingView />

  const list = (workloads || [])
    .filter(w => !nsFilter || w.namespace === nsFilter)
    .filter(w => !riskFilter || w.overall_risk === riskFilter)
    .filter(w => !search || w.name?.toLowerCase().includes(search.toLowerCase()) || w.namespace?.toLowerCase().includes(search.toLowerCase()))
    .sort((a, b) => {
      let va = a[sortCol], vb = b[sortCol]
      if (typeof va === 'string') va = va.toLowerCase()
      if (typeof vb === 'string') vb = vb.toLowerCase()
      if (va < vb) return sortDir === 'asc' ? -1 : 1
      if (va > vb) return sortDir === 'asc' ? 1 : -1
      return 0
    })

  const handleSort = (col) => {
    if (sortCol === col) setSortDir(d => d === 'asc' ? 'desc' : 'asc')
    else { setSortCol(col); setSortDir('desc') }
  }

  const uniqueNs = [...new Set((workloads || []).map(w => w.namespace).filter(Boolean))].sort()

  const thStyle = (col) => ({
    padding: '10px 12px', textAlign: 'left', fontFamily: FONT.mono, fontSize: 11,
    color: sortCol === col ? C.cyan : C.dim, cursor: 'pointer', userSelect: 'none',
    textTransform: 'uppercase', letterSpacing: '.5px', borderBottom: `1px solid ${C.border}`,
    whiteSpace: 'nowrap',
  })

  const tdStyle = {
    padding: '10px 12px', fontFamily: FONT.mono, fontSize: 12, color: C.text,
    borderBottom: `1px solid ${C.border}`,
  }

  return (
    <div>
      {/* Filters */}
      <div style={{ display: 'flex', gap: 12, marginBottom: 20, flexWrap: 'wrap', alignItems: 'center' }}>
        <input
          type="text" placeholder="Search workloads…" value={search}
          onChange={e => setSearch(e.target.value)}
          style={{
            background: C.bg, border: `1px solid ${C.border}`, borderRadius: 6,
            padding: '8px 14px', color: C.text, fontFamily: FONT.mono, fontSize: 13,
            outline: 'none', flex: '1 1 200px', minWidth: 180,
          }}
        />
        <select value={nsFilter} onChange={e => setNsFilter(e.target.value)} style={selectStyle()}>
          <option value="">All Namespaces</option>
          {uniqueNs.map(ns => <option key={ns} value={ns}>{ns}</option>)}
        </select>
        <select value={riskFilter} onChange={e => setRiskFilter(e.target.value)} style={selectStyle()}>
          <option value="">All Risks</option>
          {['CRITICAL', 'HIGH', 'MEDIUM', 'LOW'].map(r => <option key={r} value={r}>{r}</option>)}
        </select>
        <div style={{ fontFamily: FONT.mono, fontSize: 12, color: C.dim }}>{list.length} results</div>
      </div>

      {/* Table */}
      <div style={{ overflowX: 'auto', background: C.surface, border: `1px solid ${C.border}`, borderRadius: 8 }}>
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr>
              <th style={thStyle('name')} onClick={() => handleSort('name')}>Name {sortCol === 'name' ? (sortDir === 'asc' ? '↑' : '↓') : ''}</th>
              <th style={thStyle('namespace')} onClick={() => handleSort('namespace')}>Namespace {sortCol === 'namespace' ? (sortDir === 'asc' ? '↑' : '↓') : ''}</th>
              <th style={thStyle('kind')} onClick={() => handleSort('kind')}>Type {sortCol === 'kind' ? (sortDir === 'asc' ? '↑' : '↓') : ''}</th>
              <th style={thStyle('replicas')} onClick={() => handleSort('replicas')}>Replicas</th>
              <th style={thStyle('qos_class')} onClick={() => handleSort('qos_class')}>QoS</th>
              <th style={thStyle('overall_waste_score')} onClick={() => handleSort('overall_waste_score')}>Waste Score {sortCol === 'overall_waste_score' ? (sortDir === 'asc' ? '↑' : '↓') : ''}</th>
              <th style={thStyle('overall_risk')} onClick={() => handleSort('overall_risk')}>Risk {sortCol === 'overall_risk' ? (sortDir === 'asc' ? '↑' : '↓') : ''}</th>
              <th style={thStyle('containers')}>Containers</th>
              <th style={thStyle('estimated_monthly_saving_usd')} onClick={() => handleSort('estimated_monthly_saving_usd')}>Est. Saving</th>
            </tr>
          </thead>
          <tbody>
            {list.length === 0 && (
              <tr><td colSpan={9} style={{ ...tdStyle, textAlign: 'center', color: C.dim, padding: 40 }}>No workloads found</td></tr>
            )}
            {list.map((w, i) => (
              <tr key={w.name + w.namespace + i} onClick={() => onSelect(w)}
                  style={{ cursor: 'pointer', transition: 'background .15s' }}
                  onMouseEnter={e => e.currentTarget.style.background = C.border + '44'}
                  onMouseLeave={e => e.currentTarget.style.background = 'transparent'}>
                <td style={{ ...tdStyle, color: C.cyan, fontWeight: 600 }}>{w.name}</td>
                <td style={tdStyle}>{w.namespace}</td>
                <td style={tdStyle}><Badge color={C.dim}>{w.kind || 'Deployment'}</Badge></td>
                <td style={tdStyle}>{w.replicas ?? '—'}</td>
                <td style={tdStyle}><Badge color={w.qos_class === 'Guaranteed' ? C.green : w.qos_class === 'Burstable' ? C.amber : C.dim}>{w.qos_class || '—'}</Badge></td>
                <td style={{ ...tdStyle, minWidth: 120 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <WasteBar value={w.overall_waste_score} />
                    <span style={{ fontSize: 11, color: C.dim, whiteSpace: 'nowrap' }}>{(w.overall_waste_score || 0).toFixed(0)}%</span>
                  </div>
                </td>
                <td style={tdStyle}><Badge color={RISK_COLORS[w.overall_risk] || C.dim}>{w.overall_risk || '—'}</Badge></td>
                <td style={tdStyle}>{w.containers?.length ?? 0}</td>
                <td style={{ ...tdStyle, color: C.green }}>{fmtDollars(w.estimated_monthly_saving_usd)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function selectStyle() {
  return {
    background: C.bg, border: `1px solid ${C.border}`, borderRadius: 6,
    padding: '8px 14px', color: C.text, fontFamily: FONT.mono, fontSize: 13,
    outline: 'none', cursor: 'pointer',
  }
}

// ═══════════════════════════════════════════════════════════════════════════════
// WORKLOAD DETAIL VIEW
// ═══════════════════════════════════════════════════════════════════════════════
function WorkloadDetail({ workload, onBack, toast }) {
  const [metrics, setMetrics] = useState(null)
  const [metricsLoading, setMetricsLoading] = useState(false)

  useEffect(() => {
    if (!workload) return
    let cancelled = false
    setMetricsLoading(true)
    apiFetch(`/api/v1/workloads/${encodeURIComponent(workload.namespace)}/${encodeURIComponent(workload.name)}/metrics`)
      .then(data => { if (!cancelled) setMetrics(data) })
      .catch(() => { if (!cancelled) setMetrics(null) })
      .finally(() => { if (!cancelled) setMetricsLoading(false) })
    return () => { cancelled = true }
  }, [workload?.namespace, workload?.name])

  if (!workload) return null

  const w = workload
  const containers = w.containers || []

  // Collect YAML patches and kubectl commands from all containers
  const yamlPatch = containers.map(c => {
    const rec = c.recommendation || {}
    return rec.yaml_patch || `# ${c.name}\nresources:\n  requests:\n    cpu: "${(rec.request?.cpu_cores || 0).toFixed(3)}"\n    memory: "${((rec.request?.memory_gib || 0) * 1024).toFixed(0)}Mi"\n  limits:\n    cpu: "${(rec.limit?.cpu_cores || 0).toFixed(3)}"\n    memory: "${((rec.limit?.memory_gib || 0) * 1024).toFixed(0)}Mi"`
  }).join('\n---\n')

  const kubectlCmd = containers.map(c => {
    const rec = c.recommendation || {}
    return rec.kubectl_cmd || ''
  }).filter(Boolean).join('\n') || `kubectl set resources ${(w.type || 'Deployment').toLowerCase()}/${w.name} -n ${w.namespace}`

  // Collect all issues from all containers
  const allIssues = containers.flatMap(c => (c.issues || []).map(issue => ({ container: c.name, issue })))

  // Compute total estimated savings from container recommendations
  const totalSaving = containers.reduce((sum, c) => sum + ((c.recommendation?.estimated_monthly_saving_usd || 0) * (w.replicas || 1)), 0)

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      {/* Back Button */}
      <button onClick={onBack} style={{ ...btnStyle(), alignSelf: 'flex-start', display: 'flex', alignItems: 'center', gap: 6 }}>
        ← Back
      </button>

      {/* Header */}
      <div style={{ background: C.surface, border: `1px solid ${C.border}`, borderRadius: 8, padding: 24 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 16, flexWrap: 'wrap' }}>
          <div style={{ fontFamily: FONT.head, fontSize: 24, fontWeight: 700, color: C.text }}>{w.name}</div>
          <Badge color={C.cyan}>{w.namespace}</Badge>
          <Badge color={C.dim}>{w.type || 'Deployment'}</Badge>
          <Badge color={C.green}>{w.replicas ?? '?'} replicas</Badge>
          <Badge color={w.qos_class === 'Guaranteed' ? C.green : w.qos_class === 'Burstable' ? C.amber : C.dim}>{w.qos_class || 'BestEffort'}</Badge>
          <Badge color={RISK_COLORS[w.overall_risk] || C.dim}>{w.overall_risk || 'LOW'}</Badge>
        </div>
        <div style={{ fontFamily: FONT.mono, fontSize: 13, color: C.dim, marginTop: 8 }}>
          Waste Score: <span style={{ color: w.overall_waste_score > 70 ? C.red : w.overall_waste_score > 40 ? C.amber : C.green, fontWeight: 700 }}>{(w.overall_waste_score || 0).toFixed(1)}%</span>
          {' · '}Estimated Savings: <span style={{ color: C.green, fontWeight: 700 }}>{fmtDollars(totalSaving)}/mo</span>
        </div>
      </div>

      {/* Container Details */}
      {containers.map((c, ci) => {
        const curReq = c.current_request || {}
        const curLim = c.current_limit || {}
        const rec = c.recommendation || {}
        const recReq = rec.request || {}
        const recLim = rec.limit || {}
        const usage = c.usage || {}
        return (
          <div key={ci} style={{ background: C.surface, border: `1px solid ${C.border}`, borderRadius: 8, padding: 24 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
              <div style={{ fontFamily: FONT.head, fontSize: 16, fontWeight: 600, color: C.cyan }}>
                Container: {c.name}
                <span style={{ fontFamily: FONT.mono, fontSize: 11, color: C.dim, marginLeft: 12 }}>{c.image}</span>
              </div>
              <div style={{ display: 'flex', gap: 8 }}>
                <Badge color={RISK_COLORS[c.risk_level] || C.dim}>{c.risk_level || 'LOW'}</Badge>
                <span style={{ fontFamily: FONT.mono, fontSize: 12, color: C.dim }}>Waste: {(c.waste_score || 0).toFixed(0)}%</span>
              </div>
            </div>

            {/* Current vs Recommended */}
            <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap', marginBottom: 20 }}>
              <div style={{ flex: '1 1 280px', background: C.bg, border: `1px solid ${C.border}`, borderRadius: 8, padding: 16 }}>
                <div style={{ fontFamily: FONT.mono, fontSize: 11, color: C.dim, textTransform: 'uppercase', letterSpacing: 1, marginBottom: 12 }}>Current</div>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
                  <MetricCell label="CPU Request" value={fmtCPU(curReq.cpu_cores)} />
                  <MetricCell label="CPU Limit" value={fmtCPU(curLim.cpu_cores)} />
                  <MetricCell label="Mem Request" value={fmtMem(curReq.memory_gib)} />
                  <MetricCell label="Mem Limit" value={fmtMem(curLim.memory_gib)} />
                </div>
              </div>
              <div style={{ flex: '1 1 280px', background: C.bg, border: `1px solid ${C.cyan}44`, borderRadius: 8, padding: 16 }}>
                <div style={{ fontFamily: FONT.mono, fontSize: 11, color: C.cyan, textTransform: 'uppercase', letterSpacing: 1, marginBottom: 12 }}>Recommended</div>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
                  <MetricCell label="CPU Request" value={fmtCPU(recReq.cpu_cores)} accent={C.cyan} />
                  <MetricCell label="CPU Limit" value={fmtCPU(recLim.cpu_cores)} accent={C.cyan} />
                  <MetricCell label="Mem Request" value={fmtMem(recReq.memory_gib)} accent={C.cyan} />
                  <MetricCell label="Mem Limit" value={fmtMem(recLim.memory_gib)} accent={C.cyan} />
                </div>
              </div>
            </div>

            {/* Usage Percentiles */}
            {usage.p50 && (
              <div style={{ marginBottom: 20 }}>
                <div style={{ fontFamily: FONT.mono, fontSize: 11, color: C.dim, textTransform: 'uppercase', letterSpacing: 1, marginBottom: 8 }}>Usage Percentiles</div>
                <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                  <thead>
                    <tr>
                      <th style={ptStyle()}>Metric</th>
                      <th style={ptStyle()}>P50</th>
                      <th style={ptStyle()}>P90</th>
                      <th style={ptStyle()}>P95</th>
                      <th style={ptStyle()}>P99</th>
                      <th style={ptStyle()}>Max</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr>
                      <td style={pcStyle()}>CPU</td>
                      <td style={pcStyle()}>{fmtCPU(usage.p50?.cpu_cores)}</td>
                      <td style={pcStyle()}>{fmtCPU(usage.p90?.cpu_cores)}</td>
                      <td style={pcStyle()}>{fmtCPU(usage.p95?.cpu_cores)}</td>
                      <td style={pcStyle()}>{fmtCPU(usage.p99?.cpu_cores)}</td>
                      <td style={pcStyle()}>{fmtCPU(usage.max?.cpu_cores)}</td>
                    </tr>
                    <tr>
                      <td style={pcStyle()}>Memory</td>
                      <td style={pcStyle()}>{fmtMem(usage.p50?.memory_gib)}</td>
                      <td style={pcStyle()}>{fmtMem(usage.p90?.memory_gib)}</td>
                      <td style={pcStyle()}>{fmtMem(usage.p95?.memory_gib)}</td>
                      <td style={pcStyle()}>{fmtMem(usage.p99?.memory_gib)}</td>
                      <td style={pcStyle()}>{fmtMem(usage.max?.memory_gib)}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            )}

            {/* Resource Usage Charts */}
            {(() => {
              const cm = metrics?.containers?.find(m => m.name === c.name)
              if (metricsLoading) return (
                <div style={{ marginBottom: 20 }}>
                  <div style={{ fontFamily: FONT.mono, fontSize: 11, color: C.dim, textTransform: 'uppercase', letterSpacing: 1, marginBottom: 8 }}>Resource Usage Over Time</div>
                  <div style={{ height: 160, display: 'flex', alignItems: 'center', justifyContent: 'center', background: C.bg, borderRadius: 8, border: `1px solid ${C.border}` }}>
                    <span style={{ fontFamily: FONT.mono, fontSize: 12, color: C.dim }}>Loading metrics…</span>
                  </div>
                </div>
              )
              if (!cm || !cm.series?.length) return null

              // Downsample if more than 200 points for performance
              const maxPts = 200
              const raw = cm.series
              const step = raw.length > maxPts ? Math.ceil(raw.length / maxPts) : 1
              const series = step > 1 ? raw.filter((_, i) => i % step === 0) : raw

              const cpuData = series.map(pt => ({
                time: new Date(pt.timestamp).getTime(),
                cpu: pt.cpu_cores,
              }))
              const memData = series.map(pt => ({
                time: new Date(pt.timestamp).getTime(),
                mem: pt.memory_gib,
              }))

              const cpuReqVal = curReq.cpu_cores || 0
              const cpuLimVal = curLim.cpu_cores || 0
              const cpuRecVal = recReq.cpu_cores || 0
              const memReqVal = curReq.memory_gib || 0
              const memLimVal = curLim.memory_gib || 0
              const memRecVal = recReq.memory_gib || 0

              const timeFmt = (ts) => {
                const d = new Date(ts)
                return `${d.getMonth()+1}/${d.getDate()} ${String(d.getHours()).padStart(2,'0')}:${String(d.getMinutes()).padStart(2,'0')}`
              }

              const chartTooltipStyle = {
                background: C.surface, border: `1px solid ${C.border}`, borderRadius: 6,
                fontFamily: FONT.mono, fontSize: 11, padding: '8px 12px',
              }

              return (
                <div style={{ marginBottom: 20 }}>
                  <div style={{ fontFamily: FONT.mono, fontSize: 11, color: C.dim, textTransform: 'uppercase', letterSpacing: 1, marginBottom: 8 }}>Resource Usage Over Time</div>
                  <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap' }}>
                    {/* CPU Chart */}
                    <div style={{ flex: '1 1 380px', background: C.bg, border: `1px solid ${C.border}`, borderRadius: 8, padding: '16px 12px 8px' }}>
                      <div style={{ fontFamily: FONT.mono, fontSize: 10, color: C.cyan, textTransform: 'uppercase', letterSpacing: 1, marginBottom: 8 }}>CPU Usage (cores)</div>
                      <ResponsiveContainer width="100%" height={180}>
                        <AreaChart data={cpuData} margin={{ top: 5, right: 10, left: 0, bottom: 5 }}>
                          <defs>
                            <linearGradient id={`cpuGrad-${ci}`} x1="0" y1="0" x2="0" y2="1">
                              <stop offset="0%" stopColor={C.cyan} stopOpacity={0.3} />
                              <stop offset="100%" stopColor={C.cyan} stopOpacity={0.02} />
                            </linearGradient>
                          </defs>
                          <CartesianGrid strokeDasharray="3 3" stroke={C.border} />
                          <XAxis dataKey="time" tick={{ fill: C.dim, fontSize: 9, fontFamily: FONT.mono }} tickFormatter={timeFmt} minTickGap={40} stroke={C.border} />
                          <YAxis tick={{ fill: C.dim, fontSize: 9, fontFamily: FONT.mono }} stroke={C.border} tickFormatter={v => v < 1 ? `${(v*1000).toFixed(0)}m` : v.toFixed(2)} />
                          <Tooltip contentStyle={chartTooltipStyle} labelFormatter={timeFmt} formatter={(v) => [v < 1 ? `${(v*1000).toFixed(0)}m` : v.toFixed(3), 'CPU']} />
                          <Area type="monotone" dataKey="cpu" stroke={C.cyan} fill={`url(#cpuGrad-${ci})`} strokeWidth={1.5} dot={false} isAnimationActive={false} />
                          {cpuReqVal > 0 && <ReferenceLine y={cpuReqVal} stroke={C.amber} strokeDasharray="6 3" label={{ value: 'Request', fill: C.amber, fontSize: 9, fontFamily: FONT.mono, position: 'right' }} />}
                          {cpuLimVal > 0 && <ReferenceLine y={cpuLimVal} stroke={C.red} strokeDasharray="6 3" label={{ value: 'Limit', fill: C.red, fontSize: 9, fontFamily: FONT.mono, position: 'right' }} />}
                          {cpuRecVal > 0 && <ReferenceLine y={cpuRecVal} stroke={C.green} strokeDasharray="4 2" label={{ value: 'Rec', fill: C.green, fontSize: 9, fontFamily: FONT.mono, position: 'right' }} />}
                        </AreaChart>
                      </ResponsiveContainer>
                    </div>

                    {/* Memory Chart */}
                    <div style={{ flex: '1 1 380px', background: C.bg, border: `1px solid ${C.border}`, borderRadius: 8, padding: '16px 12px 8px' }}>
                      <div style={{ fontFamily: FONT.mono, fontSize: 10, color: C.purple, textTransform: 'uppercase', letterSpacing: 1, marginBottom: 8 }}>Memory Usage (GiB)</div>
                      <ResponsiveContainer width="100%" height={180}>
                        <AreaChart data={memData} margin={{ top: 5, right: 10, left: 0, bottom: 5 }}>
                          <defs>
                            <linearGradient id={`memGrad-${ci}`} x1="0" y1="0" x2="0" y2="1">
                              <stop offset="0%" stopColor={C.purple} stopOpacity={0.3} />
                              <stop offset="100%" stopColor={C.purple} stopOpacity={0.02} />
                            </linearGradient>
                          </defs>
                          <CartesianGrid strokeDasharray="3 3" stroke={C.border} />
                          <XAxis dataKey="time" tick={{ fill: C.dim, fontSize: 9, fontFamily: FONT.mono }} tickFormatter={timeFmt} minTickGap={40} stroke={C.border} />
                          <YAxis tick={{ fill: C.dim, fontSize: 9, fontFamily: FONT.mono }} stroke={C.border} tickFormatter={v => v >= 1 ? `${v.toFixed(1)}` : `${(v*1024).toFixed(0)}Mi`} />
                          <Tooltip contentStyle={chartTooltipStyle} labelFormatter={timeFmt} formatter={(v) => [v >= 1 ? `${v.toFixed(3)} GiB` : `${(v*1024).toFixed(0)} MiB`, 'Memory']} />
                          <Area type="monotone" dataKey="mem" stroke={C.purple} fill={`url(#memGrad-${ci})`} strokeWidth={1.5} dot={false} isAnimationActive={false} />
                          {memReqVal > 0 && <ReferenceLine y={memReqVal} stroke={C.amber} strokeDasharray="6 3" label={{ value: 'Request', fill: C.amber, fontSize: 9, fontFamily: FONT.mono, position: 'right' }} />}
                          {memLimVal > 0 && <ReferenceLine y={memLimVal} stroke={C.red} strokeDasharray="6 3" label={{ value: 'Limit', fill: C.red, fontSize: 9, fontFamily: FONT.mono, position: 'right' }} />}
                          {memRecVal > 0 && <ReferenceLine y={memRecVal} stroke={C.green} strokeDasharray="4 2" label={{ value: 'Rec', fill: C.green, fontSize: 9, fontFamily: FONT.mono, position: 'right' }} />}
                        </AreaChart>
                      </ResponsiveContainer>
                    </div>
                  </div>
                  {/* Chart Legend */}
                  <div style={{ display: 'flex', gap: 20, marginTop: 8, flexWrap: 'wrap' }}>
                    <span style={{ fontFamily: FONT.mono, fontSize: 10, color: C.dim, display: 'flex', alignItems: 'center', gap: 4 }}>
                      <span style={{ width: 16, height: 2, background: C.amber, display: 'inline-block' }} /> Request
                    </span>
                    <span style={{ fontFamily: FONT.mono, fontSize: 10, color: C.dim, display: 'flex', alignItems: 'center', gap: 4 }}>
                      <span style={{ width: 16, height: 2, background: C.red, display: 'inline-block' }} /> Limit
                    </span>
                    <span style={{ fontFamily: FONT.mono, fontSize: 10, color: C.dim, display: 'flex', alignItems: 'center', gap: 4 }}>
                      <span style={{ width: 16, height: 2, background: C.green, display: 'inline-block' }} /> Recommended
                    </span>
                  </div>
                </div>
              )
            })()}
          </div>
        )
      })}

      {/* YAML Patch */}
      <div style={{ background: C.surface, border: `1px solid ${C.border}`, borderRadius: 8, padding: 24 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
          <div style={{ fontFamily: FONT.head, fontSize: 14, fontWeight: 600, color: C.text }}>YAML Patch</div>
          <button onClick={() => { copyToClipboard(yamlPatch); toast('Copied YAML to clipboard', 'success') }} style={btnStyle()}>Copy</button>
        </div>
        <pre style={{
          background: C.bg, border: `1px solid ${C.border}`, borderRadius: 6,
          padding: 16, fontFamily: FONT.mono, fontSize: 12, color: C.cyan,
          overflow: 'auto', maxHeight: 300, margin: 0, whiteSpace: 'pre-wrap',
        }}>{yamlPatch}</pre>
      </div>

      {/* Kubectl Command */}
      <div style={{ background: C.surface, border: `1px solid ${C.border}`, borderRadius: 8, padding: 24 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
          <div style={{ fontFamily: FONT.head, fontSize: 14, fontWeight: 600, color: C.text }}>kubectl Command</div>
          <button onClick={() => { copyToClipboard(kubectlCmd); toast('Copied command to clipboard', 'success') }} style={btnStyle()}>Copy</button>
        </div>
        <pre style={{
          background: C.bg, border: `1px solid ${C.border}`, borderRadius: 6,
          padding: 16, fontFamily: FONT.mono, fontSize: 12, color: C.green,
          overflow: 'auto', margin: 0, whiteSpace: 'pre-wrap',
        }}>{kubectlCmd}</pre>
      </div>

      {/* Issues */}
      {allIssues.length > 0 && (
        <div style={{ background: C.surface, border: `1px solid ${C.border}`, borderRadius: 8, padding: 24 }}>
          <div style={{ fontFamily: FONT.head, fontSize: 14, fontWeight: 600, color: C.text, marginBottom: 16 }}>Issues</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            {allIssues.map((item, ii) => (
              <div key={ii} style={{
                display: 'flex', alignItems: 'center', gap: 12, padding: '10px 14px',
                background: C.bg, border: `1px solid ${C.border}`, borderRadius: 6,
              }}>
                <span style={{
                  width: 8, height: 8, borderRadius: '50%', flexShrink: 0,
                  background: item.issue.includes('oom') || item.issue.includes('under') ? C.red : item.issue.includes('no_limits') ? C.amber : C.dim,
                }} />
                <span style={{ fontFamily: FONT.mono, fontSize: 12, color: C.text }}>{item.container}: {item.issue}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

function MetricCell({ label, value, accent }) {
  return (
    <div>
      <div style={{ fontFamily: FONT.mono, fontSize: 10, color: C.dim, marginBottom: 2 }}>{label}</div>
      <div style={{ fontFamily: FONT.mono, fontSize: 14, fontWeight: 600, color: accent || C.text }}>{value}</div>
    </div>
  )
}

function ptStyle() {
  return { padding: '6px 10px', textAlign: 'left', fontFamily: FONT.mono, fontSize: 10, color: C.dim, textTransform: 'uppercase', letterSpacing: '.5px', borderBottom: `1px solid ${C.border}` }
}

function pcStyle() {
  return { padding: '6px 10px', fontFamily: FONT.mono, fontSize: 12, color: C.text, borderBottom: `1px solid ${C.border}` }
}

// ═══════════════════════════════════════════════════════════════════════════════
// RECOMMENDATIONS VIEW
// ═══════════════════════════════════════════════════════════════════════════════
function Recommendations({ workloads, loading, onSelect }) {
  const [nsFilter, setNsFilter] = useState('')
  const [riskFilter, setRiskFilter] = useState('')

  if (loading) return <LoadingView />

  const wl = (workloads || [])
    .filter(w => !nsFilter || w.namespace === nsFilter)
    .filter(w => !riskFilter || w.overall_risk === riskFilter)

  const uniqueNs = [...new Set((workloads || []).map(w => w.namespace).filter(Boolean))].sort()
  const riskOrder = ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW']
  const grouped = {}
  riskOrder.forEach(r => { grouped[r] = [] })
  wl.forEach(w => {
    const r = w.overall_risk || 'LOW'
    if (!grouped[r]) grouped[r] = []
    grouped[r].push(w)
  })

  return (
    <div>
      {/* Filters */}
      <div style={{ display: 'flex', gap: 12, marginBottom: 24, flexWrap: 'wrap', alignItems: 'center' }}>
        <select value={nsFilter} onChange={e => setNsFilter(e.target.value)} style={selectStyle()}>
          <option value="">All Namespaces</option>
          {uniqueNs.map(ns => <option key={ns} value={ns}>{ns}</option>)}
        </select>
        <select value={riskFilter} onChange={e => setRiskFilter(e.target.value)} style={selectStyle()}>
          <option value="">All Risks</option>
          {riskOrder.map(r => <option key={r} value={r}>{r}</option>)}
        </select>
      </div>

      {riskOrder.map(risk => {
        const items = grouped[risk]
        if (items.length === 0) return null
        return (
          <div key={risk} style={{ marginBottom: 32 }}>
            <div style={{
              fontFamily: FONT.head, fontSize: 16, fontWeight: 700,
              color: RISK_COLORS[risk], marginBottom: 12,
              display: 'flex', alignItems: 'center', gap: 8,
            }}>
              <span style={{ width: 10, height: 10, borderRadius: '50%', background: RISK_COLORS[risk] }} />
              {risk} ({items.length})
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))', gap: 12 }}>
              {items.map((w, i) => {
                return (
                <div key={w.name + w.namespace + i} onClick={() => onSelect(w)} style={{
                  background: C.surface, border: `1px solid ${C.border}`, borderRadius: 8,
                  padding: 16, cursor: 'pointer', transition: 'border-color .2s',
                }}
                onMouseEnter={e => { e.currentTarget.style.borderColor = RISK_COLORS[risk] }}
                onMouseLeave={e => { e.currentTarget.style.borderColor = C.border }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 8 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <div>
                        <div style={{ fontFamily: FONT.mono, fontSize: 13, fontWeight: 600, color: C.text }}>{w.name}</div>
                        <div style={{ fontFamily: FONT.mono, fontSize: 11, color: C.dim, marginTop: 2 }}>{w.namespace}</div>
                      </div>
                    </div>
                    <Badge color={RISK_COLORS[risk]}>{risk}</Badge>
                  </div>
                  <div style={{ display: 'flex', gap: 16, marginBottom: 8 }}>
                    <div>
                      <div style={{ fontFamily: FONT.mono, fontSize: 10, color: C.dim }}>WASTE</div>
                      <div style={{ fontFamily: FONT.mono, fontSize: 16, fontWeight: 700, color: RISK_COLORS[risk] }}>{(w.overall_waste_score || 0).toFixed(0)}%</div>
                    </div>
                    <div>
                      <div style={{ fontFamily: FONT.mono, fontSize: 10, color: C.dim }}>SAVINGS</div>
                      <div style={{ fontFamily: FONT.mono, fontSize: 16, fontWeight: 700, color: C.green }}>{fmtDollars(w.estimated_monthly_saving_usd)}</div>
                    </div>
                  </div>
                  <WasteBar value={w.overall_waste_score} />
                  {(w.issues || []).length > 0 && (
                    <div style={{ marginTop: 10, display: 'flex', flexDirection: 'column', gap: 4 }}>
                      {(w.issues || []).slice(0, 3).map((iss, ii) => (
                        <div key={ii} style={{ fontFamily: FONT.mono, fontSize: 11, color: C.dim, display: 'flex', alignItems: 'center', gap: 6 }}>
                          <span style={{ width: 4, height: 4, borderRadius: '50%', background: C.amber, flexShrink: 0 }} />
                          {typeof iss === 'string' ? iss : (iss.message || iss.description)}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )})}
            </div>
          </div>
        )
      })}

      {wl.length === 0 && (
        <div style={{ textAlign: 'center', padding: 60, fontFamily: FONT.mono, fontSize: 14, color: C.dim }}>
          No recommendations matching filters
        </div>
      )}
    </div>
  )
}

// ═══════════════════════════════════════════════════════════════════════════════
// SETTINGS VIEW
// ═══════════════════════════════════════════════════════════════════════════════
function Settings({ toast }) {
  const [config, setConfig] = useState({
    prometheus_url: 'http://prometheus:9090',
    lookback_window: '7d',
    headroom_factor: 15,
    spike_percentile: 'P95',
    cost_cpu_hour: 0.031611,
    cost_gib_hour: 0.004237,
    namespace_exclusions: 'kube-system,kube-public',
    mock_mode: false,
  })
  const [testing, setTesting] = useState(false)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    apiFetch('/api/v1/config')
      .then(data => {
        if (data) setConfig(prev => ({ ...prev, ...data }))
      })
      .catch(() => {})
  }, [])

  const update = (key, val) => setConfig(prev => ({ ...prev, [key]: val }))

  const testConnection = async () => {
    setTesting(true)
    try {
      await apiFetch('/api/v1/config/test-prometheus', {
        method: 'POST',
        body: JSON.stringify({ url: config.prometheus_url }),
      })
      toast('Prometheus connection successful', 'success')
    } catch {
      toast('Failed to connect to Prometheus', 'error')
    } finally {
      setTesting(false)
    }
  }

  const saveConfig = async () => {
    setSaving(true)
    try {
      await apiFetch('/api/v1/config', {
        method: 'PUT',
        body: JSON.stringify(config),
      })
      toast('Configuration saved', 'success')
    } catch {
      toast('Failed to save configuration', 'error')
    } finally {
      setSaving(false)
    }
  }

  const labelStyle = {
    fontFamily: FONT.mono, fontSize: 11, color: C.dim,
    textTransform: 'uppercase', letterSpacing: '.5px', marginBottom: 6, display: 'block',
  }

  const inputStyle = {
    background: C.bg, border: `1px solid ${C.border}`, borderRadius: 6,
    padding: '10px 14px', color: C.text, fontFamily: FONT.mono, fontSize: 13,
    outline: 'none', width: '100%', boxSizing: 'border-box',
  }

  const groupStyle = {
    background: C.surface, border: `1px solid ${C.border}`, borderRadius: 8,
    padding: 24, display: 'flex', flexDirection: 'column', gap: 20,
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20, maxWidth: 640 }}>
      <div style={{ fontFamily: FONT.head, fontSize: 22, fontWeight: 700, color: C.text }}>Settings</div>

      {/* Prometheus */}
      <div style={groupStyle}>
        <div style={{ fontFamily: FONT.head, fontSize: 14, fontWeight: 600, color: C.text }}>Data Source</div>
        <div>
          <label style={labelStyle}>Prometheus URL</label>
          <div style={{ display: 'flex', gap: 8 }}>
            <input style={{ ...inputStyle, flex: 1 }} value={config.prometheus_url} onChange={e => update('prometheus_url', e.target.value)} />
            <button onClick={testConnection} disabled={testing} style={btnStyle()}>{testing ? 'Testing…' : 'Test Connection'}</button>
          </div>
        </div>
        <div>
          <label style={labelStyle}>Lookback Window</label>
          <select value={config.lookback_window} onChange={e => update('lookback_window', e.target.value)} style={{ ...inputStyle, cursor: 'pointer' }}>
            {['1d', '3d', '7d', '14d', '30d'].map(v => <option key={v} value={v}>{v}</option>)}
          </select>
        </div>
      </div>

      {/* Analysis */}
      <div style={groupStyle}>
        <div style={{ fontFamily: FONT.head, fontSize: 14, fontWeight: 600, color: C.text }}>Analysis Parameters</div>
        <div>
          <label style={labelStyle}>Headroom Factor: {config.headroom_factor}%</label>
          <input type="range" min={5} max={50} value={config.headroom_factor} onChange={e => update('headroom_factor', Number(e.target.value))}
            style={{ width: '100%', accentColor: C.cyan }} />
          <div style={{ display: 'flex', justifyContent: 'space-between', fontFamily: FONT.mono, fontSize: 10, color: C.dim }}>
            <span>5%</span><span>50%</span>
          </div>
        </div>
        <div>
          <label style={labelStyle}>Spike Percentile</label>
          <div style={{ display: 'flex', gap: 8 }}>
            {['P90', 'P95', 'P99', 'Max'].map(p => (
              <label key={p} style={{
                display: 'flex', alignItems: 'center', gap: 6, padding: '8px 14px',
                background: config.spike_percentile === p ? C.cyan + '22' : C.bg,
                border: `1px solid ${config.spike_percentile === p ? C.cyan : C.border}`,
                borderRadius: 6, cursor: 'pointer', fontFamily: FONT.mono, fontSize: 12,
                color: config.spike_percentile === p ? C.cyan : C.text,
              }}>
                <input type="radio" name="spike" value={p} checked={config.spike_percentile === p}
                  onChange={() => update('spike_percentile', p)} style={{ display: 'none' }} />
                {p}
              </label>
            ))}
          </div>
        </div>
      </div>

      {/* Cost */}
      <div style={groupStyle}>
        <div style={{ fontFamily: FONT.head, fontSize: 14, fontWeight: 600, color: C.text }}>Cost Model</div>
        <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap' }}>
          <div style={{ flex: '1 1 200px' }}>
            <label style={labelStyle}>Cost per CPU Hour ($)</label>
            <input type="number" step="0.001" value={config.cost_cpu_hour} onChange={e => update('cost_cpu_hour', Number(e.target.value))} style={inputStyle} />
          </div>
          <div style={{ flex: '1 1 200px' }}>
            <label style={labelStyle}>Cost per GiB Hour ($)</label>
            <input type="number" step="0.001" value={config.cost_gib_hour} onChange={e => update('cost_gib_hour', Number(e.target.value))} style={inputStyle} />
          </div>
        </div>
      </div>

      {/* Exclusions */}
      <div style={groupStyle}>
        <div style={{ fontFamily: FONT.head, fontSize: 14, fontWeight: 600, color: C.text }}>Filters</div>
        <div>
          <label style={labelStyle}>Namespace Exclusions (comma separated)</label>
          <input style={inputStyle} value={config.namespace_exclusions} onChange={e => update('namespace_exclusions', e.target.value)} placeholder="kube-system, kube-public" />
        </div>
        <div>
          <label style={labelStyle}>Mock Mode</label>
          <div onClick={() => update('mock_mode', !config.mock_mode)} style={{
            width: 48, height: 26, borderRadius: 13, cursor: 'pointer',
            background: config.mock_mode ? C.cyan : C.muted, transition: 'background .2s',
            position: 'relative',
          }}>
            <div style={{
              width: 20, height: 20, borderRadius: '50%', background: '#fff',
              position: 'absolute', top: 3, left: config.mock_mode ? 25 : 3,
              transition: 'left .2s',
            }} />
          </div>
        </div>
      </div>

      {/* Save */}
      <button onClick={saveConfig} disabled={saving} style={btnSolid()}>
        {saving ? 'Saving…' : 'Save Configuration'}
      </button>
    </div>
  )
}

// ═══════════════════════════════════════════════════════════════════════════════
// MAIN APP
// ═══════════════════════════════════════════════════════════════════════════════
export default function App() {
  const [view, setView] = useState('overview')
  const [summary, setSummary] = useState(null)
  const [workloads, setWorkloads] = useState([])
  const [namespaces, setNamespaces] = useState([])
  const [selectedWorkload, setSelectedWorkload] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const { toasts, add: toast } = useToast()
  const refreshRef = useRef(null)

  const fetchData = useCallback(async () => {
    try {
      const [summaryRes, workloadsRes, nsRes] = await Promise.allSettled([
        apiFetch('/api/v1/cluster/summary'),
        apiFetch('/api/v1/workloads?page_size=100'),
        apiFetch('/api/v1/namespaces'),
      ])
      if (summaryRes.status === 'fulfilled') setSummary(summaryRes.value)
      if (workloadsRes.status === 'fulfilled') {
        const val = workloadsRes.value
        setWorkloads(Array.isArray(val) ? val : val?.data || val?.workloads || [])
      }
      if (nsRes.status === 'fulfilled') setNamespaces(Array.isArray(nsRes.value) ? nsRes.value : nsRes.value?.namespaces || [])
      setError(null)
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchData()
    refreshRef.current = setInterval(fetchData, 30000)
    return () => clearInterval(refreshRef.current)
  }, [fetchData])

  const handleSelectWorkload = async (w) => {
    try {
      const analysis = await apiFetch(`/api/v1/workloads/${encodeURIComponent(w.namespace)}/${encodeURIComponent(w.name)}/analysis`)
      setSelectedWorkload(analysis)
    } catch {
      setSelectedWorkload(w)
    }
    setView('detail')
  }

  const handleBack = () => {
    setSelectedWorkload(null)
    setView('workloads')
  }

  const renderView = () => {
    if (error && !workloads.length) return <ErrorView message={error} onRetry={fetchData} />
    switch (view) {
      case 'overview':
        return <ClusterOverview summary={summary} workloads={workloads} namespaces={namespaces} loading={loading} />
      case 'workloads':
        return <WorkloadExplorer workloads={workloads} namespaces={namespaces} loading={loading} onSelect={handleSelectWorkload} />
      case 'detail':
        return <WorkloadDetail workload={selectedWorkload} onBack={handleBack} toast={toast} />
      case 'recommend':
        return <Recommendations workloads={workloads} loading={loading} onSelect={handleSelectWorkload} />
      case 'settings':
        return <Settings toast={toast} />
      default:
        return <ClusterOverview summary={summary} workloads={workloads} namespaces={namespaces} loading={loading} />
    }
  }

  return (
    <>
      <style>{`
        *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
        body {
          background: ${C.bg};
          color: ${C.text};
          font-family: ${FONT.mono};
          -webkit-font-smoothing: antialiased;
        }
        ::-webkit-scrollbar { width: 6px; height: 6px; }
        ::-webkit-scrollbar-track { background: ${C.bg}; }
        ::-webkit-scrollbar-thumb { background: ${C.muted}; border-radius: 3px; }
        ::selection { background: ${C.cyan}44; color: ${C.text}; }
        @keyframes pulse { 0%, 100% { opacity: .5; } 50% { opacity: 1; } }
        @keyframes slideIn { from { transform: translateX(60px); opacity: 0; } to { transform: translateX(0); opacity: 1; } }
        button:hover { filter: brightness(1.2); }
        button:active { transform: scale(.98); }
      `}</style>

      <ToastContainer toasts={toasts} />

      {/* Grid texture overlay */}
      <div style={{
        position: 'fixed', inset: 0, zIndex: 0, pointerEvents: 'none',
        backgroundImage: `
          linear-gradient(${C.border}22 1px, transparent 1px),
          linear-gradient(90deg, ${C.border}22 1px, transparent 1px)
        `,
        backgroundSize: '40px 40px',
      }} />

      <div style={{ display: 'flex', minHeight: '100vh', position: 'relative', zIndex: 1 }}>
        {/* Sidebar */}
        <nav style={{
          width: 220, background: C.surface, borderRight: `1px solid ${C.border}`,
          display: 'flex', flexDirection: 'column', padding: '20px 0', flexShrink: 0,
        }}>
          {/* Logo */}
          <div style={{
            padding: '0 20px 24px', borderBottom: `1px solid ${C.border}`, marginBottom: 8,
          }}>
            <div style={{ fontFamily: FONT.head, fontSize: 22, fontWeight: 800, color: C.cyan, letterSpacing: '-0.5px' }}>
              KUBEACLE
            </div>
            <div style={{ fontFamily: FONT.mono, fontSize: 10, color: C.dim, letterSpacing: 2, textTransform: 'uppercase', marginTop: 2 }}>
              Rightsizer
            </div>
          </div>

          {/* Nav Items */}
          {NAV_ITEMS.map(item => {
            const active = view === item.id || (view === 'detail' && item.id === 'workloads')
            return (
              <button key={item.id} onClick={() => { setView(item.id); if (item.id !== 'workloads') setSelectedWorkload(null) }}
                style={{
                  display: 'flex', alignItems: 'center', gap: 10, padding: '12px 20px',
                  background: active ? C.cyan + '11' : 'transparent',
                  border: 'none', borderLeft: `3px solid ${active ? C.cyan : 'transparent'}`,
                  color: active ? C.cyan : C.dim, cursor: 'pointer',
                  fontFamily: FONT.mono, fontSize: 13, fontWeight: active ? 600 : 400,
                  textAlign: 'left', transition: 'all .15s', width: '100%',
                  filter: 'none',
                }}>
                <span style={{ fontSize: 16, width: 22, textAlign: 'center' }}>{item.icon}</span>
                {item.label}
              </button>
            )
          })}

          {/* Spacer */}
          <div style={{ flex: 1 }} />

          {/* Status */}
          <div style={{ padding: '16px 20px', borderTop: `1px solid ${C.border}` }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span style={{
                width: 8, height: 8, borderRadius: '50%',
                background: error ? C.red : C.green,
                boxShadow: `0 0 8px ${error ? C.red : C.green}`,
              }} />
              <span style={{ fontFamily: FONT.mono, fontSize: 11, color: C.dim }}>
                {error ? 'Disconnected' : 'Connected'}
              </span>
            </div>
            <div style={{ fontFamily: FONT.mono, fontSize: 10, color: C.muted, marginTop: 4 }}>
              Auto-refresh: 30s
            </div>
          </div>
        </nav>

        {/* Main Content */}
        <main style={{ flex: 1, padding: 32, overflow: 'auto' }}>
          {/* Header */}
          <div style={{ marginBottom: 28, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <div>
              <h1 style={{ fontFamily: FONT.head, fontSize: 26, fontWeight: 700, color: C.text, margin: 0 }}>
                {view === 'overview' && 'Cluster Overview'}
                {view === 'workloads' && 'Workload Explorer'}
                {view === 'detail' && (selectedWorkload?.name || 'Workload Detail')}
                {view === 'recommend' && 'Recommendations'}
                {view === 'settings' && 'Settings'}
              </h1>
              <p style={{ fontFamily: FONT.mono, fontSize: 12, color: C.dim, marginTop: 4 }}>
                {view === 'overview' && 'Real-time resource optimization insights'}
                {view === 'workloads' && 'Browse and analyze Kubernetes workloads'}
                {view === 'detail' && 'Detailed resource analysis and recommendations'}
                {view === 'recommend' && 'Prioritized optimization recommendations'}
                {view === 'settings' && 'Configure analysis parameters and data sources'}
              </p>
            </div>
            {view !== 'settings' && (
              <button onClick={fetchData} style={btnStyle()}>↻ Refresh</button>
            )}
          </div>
          {renderView()}
        </main>
      </div>
    </>
  )
}
