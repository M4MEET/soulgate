import { useState, useEffect, useCallback } from 'react';
import { fetchHealth, fetchSessions, type HealthData, type SessionData } from '../lib/api';

export function useHealth(intervalMs = 5000) {
  const [health, setHealth] = useState<HealthData | null>(null);
  const [sessions, setSessions] = useState<SessionData[]>([]);
  const [connected, setConnected] = useState(false);

  const refresh = useCallback(async () => {
    try {
      const [h, s] = await Promise.all([fetchHealth(), fetchSessions()]);
      setHealth(h);
      setSessions(s);
      setConnected(true);
    } catch {
      setConnected(false);
    }
  }, []);

  useEffect(() => {
    refresh();
    const id = setInterval(refresh, intervalMs);
    return () => clearInterval(id);
  }, [refresh, intervalMs]);

  return { health, sessions, connected, refresh };
}
