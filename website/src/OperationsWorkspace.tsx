import { useEffect, useState } from "react";
import { AlertTriangle, ArrowDownRight, ArrowUpRight, BarChart3, Boxes, Download, Gauge, GitFork, Layers3, LoaderCircle, LockKeyhole, Maximize2, RefreshCw, SearchX, ServerCog, UsersRound, ZoomIn, ZoomOut } from "lucide-react";
import { datadockApi, type Connection, type OperationsMetric, type OperationsSnapshot } from "./api";
import "./operations.css";

type Area = "overview" | "sessions" | "locks" | "performance" | "erd";
type State = { loading: boolean; unavailable: boolean; error: string; snapshot?: OperationsSnapshot };
const empty: State = { loading: false, unavailable: false, error: "" };
const nav: Array<{ id: Area; label: string; icon: typeof Gauge }> = [
  { id: "overview", label: "Overview", icon: Gauge }, { id: "sessions", label: "Sessions", icon: UsersRound }, { id: "locks", label: "Locks", icon: LockKeyhole }, { id: "performance", label: "Performance", icon: BarChart3 }, { id: "erd", label: "ER diagram", icon: GitFork },
];
const fallback: Record<Exclude<Area, "erd">, OperationsMetric[]> = {
  overview: [{ label: "Connection", value: "Healthy", detail: "Ready", trend: "up" }, { label: "Active sessions", value: "—" }, { label: "Storage", value: "—" }, { label: "Cache hit", value: "—" }],
  sessions: [{ label: "Active", value: "—" }, { label: "Idle", value: "—" }, { label: "Max connections", value: "—" }],
  locks: [{ label: "Waiting", value: "—" }, { label: "Blocking", value: "—" }, { label: "Deadlocks", value: "—" }],
  performance: [{ label: "Queries / sec", value: "—" }, { label: "Avg duration", value: "—" }, { label: "Slow queries", value: "—" }],
};

function Capability({ title, detail, retry }: { title: string; detail: string; retry?: () => void }) {
  return <div className="operations-capability"><div><SearchX size={20} /></div><strong>{title}</strong><p>{detail}</p>{retry && <button className="button secondary" onClick={retry}><RefreshCw size={14} /> Retry</button>}</div>;
}

function Diagram() {
  const [zoom, setZoom] = useState(100);
  return <div className="erd-shell"><div className="erd-toolbar"><span><Boxes size={15} /> public schema <small>{zoom}%</small></span><div><button onClick={() => setZoom((z) => Math.max(60, z - 10))}><ZoomOut size={15} /></button><button onClick={() => setZoom((z) => Math.min(150, z + 10))}><ZoomIn size={15} /></button><button onClick={() => setZoom(100)}><Maximize2 size={15} /></button><button className="erd-export"><Download size={14} /> Export PNG</button></div></div><div className="erd-canvas"><div className="erd-preview" style={{ transform: `scale(${zoom / 100})` }}><svg viewBox="0 0 500 250" aria-hidden="true"><path d="M145 110 C210 105 230 65 300 85"/><path d="M145 175 C220 175 225 155 300 160"/></svg><Node className="node-users" name="users" fields={["PK  id · uuid", "email · varchar", "created_at · timestamptz"]}/><Node className="node-projects" name="projects" fields={["PK  id · uuid", "FK  owner_id · uuid", "name · varchar"]}/><Node className="node-orgs" name="organizations" fields={["PK  id · uuid", "name · varchar"]}/></div><div className="erd-caption"><Layers3 size={17}/><strong>Relationship preview</strong><span>Schema relationships appear when foreign-key metadata is available.</span></div></div></div>;
}
function Node({ className, name, fields }: { className: string; name: string; fields: string[] }) { return <article className={`erd-node ${className}`}><header><ServerCog size={13}/>{name}</header>{fields.map((field) => <span key={field}>{field}</span>)}</article>; }

export function OperationsWorkspace({ connection }: { connection?: Connection }) {
  const [area, setArea] = useState<Area>("overview");
  const [state, setState] = useState<State>(empty);
  async function refresh() {
    if (!connection || area === "erd") return;
    setState({ loading: true, unavailable: false, error: "" });
    const fn = area === "overview" ? datadockApi.getDashboard : area === "sessions" ? datadockApi.getSessions : area === "locks" ? datadockApi.getLocks : datadockApi.getPerformance;
    try { setState({ loading: false, unavailable: false, error: "", snapshot: await fn(connection.id) }); }
    catch (error) { setState({ loading: false, unavailable: true, error: error instanceof Error ? error.message : "Capability is unavailable" }); }
  }
  useEffect(() => { void refresh(); }, [area, connection?.id]);
  const title = nav.find((item) => item.id === area)?.label || "Operations";
  const metrics = state.snapshot?.metrics?.length ? state.snapshot.metrics : fallback[area === "erd" ? "overview" : area];
  return <section className="operations-workspace"><div className="operations-heading"><div><div className="eyebrow">OPERATIONS CENTER</div><h1>{title}</h1><p>{connection ? <><i className="query-online"/> {connection.name} <b>·</b> {connection.engine}</> : "Choose a connection to inspect its operational state."}</p></div><button className="button secondary" disabled={!connection || area === "erd" || state.loading} onClick={() => void refresh()}><RefreshCw size={14} className={state.loading ? "spin" : ""}/> Refresh</button></div><nav className="operations-tabs">{nav.map((item) => { const Icon = item.icon; return <button className={area === item.id ? "active" : ""} onClick={() => setArea(item.id)} key={item.id}><Icon size={14}/>{item.label}</button>; })}</nav>{!connection ? <Capability title="No connection selected" detail="Connect to a database to load live operations and schema metadata."/> : area === "erd" ? <Diagram/> : state.loading ? <div className="operations-loading"><LoaderCircle className="spin" size={19}/> Loading live database metrics…</div> : state.unavailable ? <Capability title={`${title} is unavailable`} detail={state.error || "This engine or API version does not provide this capability yet."} retry={() => void refresh()}/> : <Panel area={area} metrics={metrics} message={state.snapshot?.message}/> }</section>;
}
function Panel({ area, metrics, message }: { area: Exclude<Area, "erd">; metrics: OperationsMetric[]; message?: string }) {
  const icon = area === "locks" ? <LockKeyhole size={17}/> : area === "sessions" ? <UsersRound size={17}/> : area === "performance" ? <BarChart3 size={17}/> : <ServerCog size={17}/>;
  return <div className="operations-panel"><div className="operations-summary"><div>{icon}<div><strong>{area === "overview" ? "Database health" : `${area[0].toUpperCase()}${area.slice(1)} monitor`}</strong><p>{message || "Live metrics are sampled from the active connection."}</p></div></div><span><i/> Live</span></div><div className="operations-metrics">{metrics.map((metric, i) => <article key={`${metric.label}-${i}`}><span>{metric.label}</span><strong>{metric.value}</strong>{metric.detail && <small className={metric.trend === "down" ? "metric-down" : "metric-up"}>{metric.trend === "down" ? <ArrowDownRight size={12}/> : <ArrowUpRight size={12}/>} {metric.detail}</small>}</article>)}</div><article className="operations-chart-card"><header><strong>{area === "locks" ? "Lock activity" : area === "performance" ? "Workload timeline" : area === "sessions" ? "Session activity" : "Connection activity"}</strong><span>Last refresh: now</span></header><div className="operations-chart"><svg viewBox="0 0 780 180" preserveAspectRatio="none"><defs><linearGradient id="opsFill" x1="0" y1="0" x2="0" y2="1"><stop stopColor="var(--accent)" stopOpacity=".35"/><stop offset="1" stopColor="var(--accent)" stopOpacity="0"/></linearGradient></defs><path className="chart-fill" d="M0,150 C75,146 90,90 150,112 S225,64 300,94 S370,130 450,74 S515,104 590,60 S670,99 780,34 L780,180 L0,180Z"/><path className="chart-line" d="M0,150 C75,146 90,90 150,112 S225,64 300,94 S370,130 450,74 S515,104 590,60 S670,99 780,34"/></svg><p><AlertTriangle size={14}/> Granular time-series retention is available when the monitoring collector is configured.</p></div></article></div>;
}
