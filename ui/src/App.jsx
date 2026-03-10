import React, { useState, useEffect, useCallback, useRef } from 'react'
import {
  PieChart, Pie, Cell, ResponsiveContainer, BarChart, Bar,
  XAxis, YAxis, Tooltip, RadialBarChart, RadialBar, Legend,
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

function fmtCPU(millicores) {
  if (millicores == null) return '—'
  if (millicores >= 1000) return (millicores / 1000).toFixed(2) + ' cores'
  return millicores + 'm'
}

function fmtMem(mib) {
  if (mib == null) return '—'
  if (mib >= 1024) return (mib / 1024).toFixed(2) + ' GiB'
  return mib + ' MiB'
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
  const memWaste = summary?.memory_waste_percent ?? 0
  const estSavings = summary?.estimated_monthly_savings ?? 0

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
    const risk = w.risk_level || 'LOW'
    if (riskCounts[risk] !== undefined) riskCounts[risk]++
  })
  const riskData = Object.entries(riskCounts).map(([name, value]) => ({ name, value }))

  // Namespace heatmap
  const nsMap = {}
  ;(workloads || []).forEach(w => {
    const ns = w.namespace || 'default'
    if (!nsMap[ns]) nsMap[ns] = { count: 0, waste: 0 }
    nsMap[ns].count++
    nsMap[ns].waste += (w.waste_score || 0)
  })
  const nsHeat = Object.entries(nsMap).map(([name, d]) => ({
    name, count: d.count, avgWaste: d.count > 0 ? d.waste / d.count : 0,
  })).sort((a, b) => b.avgWaste - a.avgWaste)

  // Recent recommendations
  const recentRecs = (workloads || [])
    .filter(w => w.risk_level === 'CRITICAL' || w.risk_level === 'HIGH')
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
                <Badge color={RISK_COLORS[w.risk_level]}>{w.risk_level}</Badge>
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
  const [sortCol, setSortCol] = useState('waste_score')
  const [sortDir, setSortDir] = useState('desc')
  const [search, setSearch] = useState('')

  if (loading) return <LoadingView />

  const list = (workloads || [])
    .filter(w => !nsFilter || w.namespace === nsFilter)
    .filter(w => !riskFilter || w.risk_level === riskFilter)
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
              <th style={thStyle('waste_score')} onClick={() => handleSort('waste_score')}>Waste Score {sortCol === 'waste_score' ? (sortDir === 'asc' ? '↑' : '↓') : ''}</th>
              <th style={thStyle('risk_level')} onClick={() => handleSort('risk_level')}>Risk {sortCol === 'risk_level' ? (sortDir === 'asc' ? '↑' : '↓') : ''}</th>
              <th style={thStyle('containers')}>Containers</th>
              <th style={thStyle('estimated_monthly_savings')} onClick={() => handleSort('estimated_monthly_savings')}>Est. Saving</th>
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
                    <WasteBar value={w.waste_score} />
                    <span style={{ fontSize: 11, color: C.dim, whiteSpace: 'nowrap' }}>{(w.waste_score || 0).toFixed(0)}%</span>
                  </div>
                </td>
                <td style={tdStyle}><Badge color={RISK_COLORS[w.risk_level] || C.dim}>{w.risk_level || '—'}</Badge></td>
                <td style={tdStyle}>{w.containers?.length ?? 0}</td>
                <td style={{ ...tdStyle, color: C.green }}>{fmtDollars(w.estimated_monthly_savings)}</td>
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
  if (!workload) return null

  const w = workload
  const containers = w.containers || []

  const yamlPatch = w.yaml_patch || containers.map(c => {
    const rec = c.recommendation || {}
    return `# ${c.name}\nresources:\n  requests:\n    cpu: "${rec.cpu_request || '?'}m"\n    memory: "${rec.memory_request || '?'}Mi"\n  limits:\n    cpu: "${rec.cpu_limit || '?'}m"\n    memory: "${rec.memory_limit || '?'}Mi"`
  }).join('\n---\n')

  const kubectlCmd = w.kubectl_command || `kubectl set resources deployment/${w.name} -n ${w.namespace} ${containers.map(c => {
    const rec = c.recommendation || {}
    return `-c ${c.name} --requests=cpu=${rec.cpu_request || '?'}m,memory=${rec.memory_request || '?'}Mi --limits=cpu=${rec.cpu_limit || '?'}m,memory=${rec.memory_limit || '?'}Mi`
  }).join(' ')}`

  const issues = w.issues || []

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
          <Badge color={C.dim}>{w.kind || 'Deployment'}</Badge>
          <Badge color={C.green}>{w.replicas ?? '?'} replicas</Badge>
          <Badge color={w.qos_class === 'Guaranteed' ? C.green : w.qos_class === 'Burstable' ? C.amber : C.dim}>{w.qos_class || 'BestEffort'}</Badge>
          <Badge color={RISK_COLORS[w.risk_level] || C.dim}>{w.risk_level || 'LOW'}</Badge>
        </div>
        <div style={{ fontFamily: FONT.mono, fontSize: 13, color: C.dim, marginTop: 8 }}>
          Waste Score: <span style={{ color: w.waste_score > 70 ? C.red : w.waste_score > 40 ? C.amber : C.green, fontWeight: 700 }}>{(w.waste_score || 0).toFixed(1)}%</span>
          {' · '}Estimated Savings: <span style={{ color: C.green, fontWeight: 700 }}>{fmtDollars(w.estimated_monthly_savings)}/mo</span>
        </div>
      </div>

      {/* Container Details */}
      {containers.map((c, ci) => {
        const cur = c.current || {}
        const rec = c.recommendation || {}
        const usage = c.usage_percentiles || {}
        return (
          <div key={ci} style={{ background: C.surface, border: `1px solid ${C.border}`, borderRadius: 8, padding: 24 }}>
            <div style={{ fontFamily: FONT.head, fontSize: 16, fontWeight: 600, color: C.cyan, marginBottom: 16 }}>
              Container: {c.name}
            </div>

            {/* Current vs Recommended */}
            <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap', marginBottom: 20 }}>
              <div style={{ flex: '1 1 280px', background: C.bg, border: `1px solid ${C.border}`, borderRadius: 8, padding: 16 }}>
                <div style={{ fontFamily: FONT.mono, fontSize: 11, color: C.dim, textTransform: 'uppercase', letterSpacing: 1, marginBottom: 12 }}>Current</div>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
                  <MetricCell label="CPU Request" value={fmtCPU(cur.cpu_request)} />
                  <MetricCell label="CPU Limit" value={fmtCPU(cur.cpu_limit)} />
                  <MetricCell label="Mem Request" value={fmtMem(cur.memory_request)} />
                  <MetricCell label="Mem Limit" value={fmtMem(cur.memory_limit)} />
                </div>
              </div>
              <div style={{ flex: '1 1 280px', background: C.bg, border: `1px solid ${C.cyan}44`, borderRadius: 8, padding: 16 }}>
                <div style={{ fontFamily: FONT.mono, fontSize: 11, color: C.cyan, textTransform: 'uppercase', letterSpacing: 1, marginBottom: 12 }}>Recommended</div>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
                  <MetricCell label="CPU Request" value={fmtCPU(rec.cpu_request)} accent={C.cyan} />
                  <MetricCell label="CPU Limit" value={fmtCPU(rec.cpu_limit)} accent={C.cyan} />
                  <MetricCell label="Mem Request" value={fmtMem(rec.memory_request)} accent={C.cyan} />
                  <MetricCell label="Mem Limit" value={fmtMem(rec.memory_limit)} accent={C.cyan} />
                </div>
              </div>
            </div>

            {/* Usage Percentiles */}
            {(usage.cpu || usage.memory) && (
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
                    {usage.cpu && (
                      <tr>
                        <td style={pcStyle()}>CPU</td>
                        <td style={pcStyle()}>{fmtCPU(usage.cpu.p50)}</td>
                        <td style={pcStyle()}>{fmtCPU(usage.cpu.p90)}</td>
                        <td style={pcStyle()}>{fmtCPU(usage.cpu.p95)}</td>
                        <td style={pcStyle()}>{fmtCPU(usage.cpu.p99)}</td>
                        <td style={pcStyle()}>{fmtCPU(usage.cpu.max)}</td>
                      </tr>
                    )}
                    {usage.memory && (
                      <tr>
                        <td style={pcStyle()}>Memory</td>
                        <td style={pcStyle()}>{fmtMem(usage.memory.p50)}</td>
                        <td style={pcStyle()}>{fmtMem(usage.memory.p90)}</td>
                        <td style={pcStyle()}>{fmtMem(usage.memory.p95)}</td>
                        <td style={pcStyle()}>{fmtMem(usage.memory.p99)}</td>
                        <td style={pcStyle()}>{fmtMem(usage.memory.max)}</td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            )}
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
      {issues.length > 0 && (
        <div style={{ background: C.surface, border: `1px solid ${C.border}`, borderRadius: 8, padding: 24 }}>
          <div style={{ fontFamily: FONT.head, fontSize: 14, fontWeight: 600, color: C.text, marginBottom: 16 }}>Issues</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            {issues.map((issue, ii) => (
              <div key={ii} style={{
                display: 'flex', alignItems: 'center', gap: 12, padding: '10px 14px',
                background: C.bg, border: `1px solid ${C.border}`, borderRadius: 6,
              }}>
                <span style={{
                  width: 8, height: 8, borderRadius: '50%', flexShrink: 0,
                  background: issue.severity === 'critical' ? C.red : issue.severity === 'high' ? C.amber : issue.severity === 'medium' ? C.purple : C.dim,
                }} />
                <span style={{ fontFamily: FONT.mono, fontSize: 12, color: C.text }}>{issue.message || issue.description || issue}</span>
                {issue.severity && <Badge color={RISK_COLORS[issue.severity?.toUpperCase()] || C.dim}>{issue.severity}</Badge>}
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
    .filter(w => !riskFilter || w.risk_level === riskFilter)

  const uniqueNs = [...new Set((workloads || []).map(w => w.namespace).filter(Boolean))].sort()
  const riskOrder = ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW']
  const grouped = {}
  riskOrder.forEach(r => { grouped[r] = [] })
  wl.forEach(w => {
    const r = w.risk_level || 'LOW'
    if (!grouped[r]) grouped[r] = []
    grouped[r].push(w)
  })

  return (
    <div>
      {/* Filters */}
      <div style={{ display: 'flex', gap: 12, marginBottom: 24, flexWrap: 'wrap' }}>
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
              {items.map((w, i) => (
                <div key={w.name + w.namespace + i} onClick={() => onSelect(w)} style={{
                  background: C.surface, border: `1px solid ${C.border}`, borderRadius: 8,
                  padding: 16, cursor: 'pointer', transition: 'border-color .2s',
                }}
                onMouseEnter={e => e.currentTarget.style.borderColor = RISK_COLORS[risk]}
                onMouseLeave={e => e.currentTarget.style.borderColor = C.border}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 8 }}>
                    <div>
                      <div style={{ fontFamily: FONT.mono, fontSize: 13, fontWeight: 600, color: C.text }}>{w.name}</div>
                      <div style={{ fontFamily: FONT.mono, fontSize: 11, color: C.dim, marginTop: 2 }}>{w.namespace}</div>
                    </div>
                    <Badge color={RISK_COLORS[risk]}>{risk}</Badge>
                  </div>
                  <div style={{ display: 'flex', gap: 16, marginBottom: 8 }}>
                    <div>
                      <div style={{ fontFamily: FONT.mono, fontSize: 10, color: C.dim }}>WASTE</div>
                      <div style={{ fontFamily: FONT.mono, fontSize: 16, fontWeight: 700, color: RISK_COLORS[risk] }}>{(w.waste_score || 0).toFixed(0)}%</div>
                    </div>
                    <div>
                      <div style={{ fontFamily: FONT.mono, fontSize: 10, color: C.dim }}>SAVINGS</div>
                      <div style={{ fontFamily: FONT.mono, fontSize: 16, fontWeight: 700, color: C.green }}>{fmtDollars(w.estimated_monthly_savings)}</div>
                    </div>
                  </div>
                  <WasteBar value={w.waste_score} />
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
              ))}
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
        apiFetch('/api/v1/workloads'),
        apiFetch('/api/v1/namespaces'),
      ])
      if (summaryRes.status === 'fulfilled') setSummary(summaryRes.value)
      if (workloadsRes.status === 'fulfilled') setWorkloads(Array.isArray(workloadsRes.value) ? workloadsRes.value : workloadsRes.value?.workloads || [])
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

  const handleSelectWorkload = (w) => {
    setSelectedWorkload(w)
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
