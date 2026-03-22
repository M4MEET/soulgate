export default function StatusDot({ connected }: { connected: boolean }) {
  return (
    <div className="status-indicator">
      <span className={`status-dot ${connected ? 'online' : 'offline'}`} />
      <span className="status-text">{connected ? 'Connected' : 'Offline'}</span>
    </div>
  );
}
