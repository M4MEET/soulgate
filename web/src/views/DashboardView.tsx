import type { HealthData, SessionData } from '../lib/api';
import { Activity, Cpu, HardDrive, Users, Zap, CheckCircle, AlertTriangle, XCircle } from 'lucide-react';

interface Props {
  health: HealthData | null;
  sessions: SessionData[];
}

function StatCard({ icon: Icon, label, value, color }: { icon: any; label: string; value: string | number; color: string }) {
  return (
    <div className="stat-card">
      <div className="stat-icon" style={{ color }}><Icon size={20} /></div>
      <div className="stat-info">
        <div className="stat-value">{value}</div>
        <div className="stat-label">{label}</div>
      </div>
    </div>
  );
}

function HealthCheck({ check }: { check: { name: string; status: string; detail?: string } }) {
  const icon = check.status === 'pass' ? <CheckCircle size={14} /> :
               check.status === 'warn' ? <AlertTriangle size={14} /> :
               <XCircle size={14} />;
  const cls = `health-${check.status}`;
  return (
    <div className={`health-check ${cls}`}>
      <span className="health-icon">{icon}</span>
      <span className="health-name">{check.name.replace(/_/g, ' ')}</span>
      {check.detail && <span className="health-detail">{check.detail}</span>}
    </div>
  );
}

export default function DashboardView({ health, sessions }: Props) {
  if (!health) {
    return <div className="view-loading"><Activity className="spin" size={24} /> Loading dashboard...</div>;
  }

  const totalClients = Object.values(health.clients).reduce((a, b) => a + b, 0);

  return (
    <div className="dashboard-view">
      <div className="dash-grid">
        <StatCard icon={Zap} label="Provider" value={health.provider || '--'} color="var(--accent)" />
        <StatCard icon={Cpu} label="Model" value={health.model || '--'} color="var(--info)" />
        <StatCard icon={Users} label="Clients" value={totalClients} color="var(--success)" />
        <StatCard icon={Activity} label="Sessions" value={health.sessions} color="var(--warning)" />
        <StatCard icon={HardDrive} label="Memory" value={`${health.memory.alloc_mb} MB`} color="var(--error)" />
        <StatCard icon={Activity} label="Goroutines" value={health.memory.goroutines} color="var(--text-secondary)" />
      </div>

      <div className="dash-panels">
        <div className="dash-panel">
          <h3>Health Checks</h3>
          <div className="health-list">
            {health.checks.map((c, i) => <HealthCheck key={i} check={c} />)}
          </div>
        </div>

        <div className="dash-panel">
          <h3>Memory</h3>
          <div className="mem-bars">
            <div className="mem-bar">
              <span className="mem-label">Allocated</span>
              <div className="mem-track">
                <div className="mem-fill" style={{ width: `${Math.min((health.memory.alloc_mb / health.memory.sys_mb) * 100, 100)}%` }} />
              </div>
              <span className="mem-value">{health.memory.alloc_mb} / {health.memory.sys_mb} MB</span>
            </div>
          </div>
          <div className="mem-stats">
            <span>GC runs: {health.memory.num_gc}</span>
            <span>Uptime: {health.uptime}</span>
          </div>
        </div>

        <div className="dash-panel">
          <h3>Active Sessions ({sessions.length})</h3>
          {sessions.length === 0 ? (
            <p className="empty-text">No active sessions</p>
          ) : (
            <div className="session-list">
              {sessions.slice(0, 10).map((s, i) => (
                <div key={i} className="session-item">
                  <span className="session-id">{s.id || s.conversation_id}</span>
                  <span className="session-channel">{s.channel}</span>
                  <span className="session-msgs">{s.message_count} msgs</span>
                </div>
              ))}
            </div>
          )}
        </div>

        <div className="dash-panel">
          <h3>Connected Clients</h3>
          <div className="client-grid">
            {Object.entries(health.clients).map(([role, count]) => (
              <div key={role} className="client-badge">
                <span className="client-role">{role}</span>
                <span className="client-count">{count}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
