import Link from 'next/link';

export default function Home() {
  return (
    <div className="container">
      <header style={{ marginBottom: '3rem' }}>
        <h2 style={{ fontSize: '2rem', fontWeight: 'bold', marginBottom: '0.5rem' }}>
          Infrastructure Control Room
        </h2>
        <p style={{ color: 'var(--text-muted)' }}>
          Manage your WireGuard + OpenClaw deployment with ease.
        </p>
      </header>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))', gap: '1.5rem', marginBottom: '3rem' }}>
        <div className="card">
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
            <h3 style={{ fontSize: '1.1rem', fontWeight: '600' }}>Deployment Status</h3>
            <span style={{ fontSize: '0.75rem', padding: '0.2rem 0.5rem', background: 'rgba(13, 148, 136, 0.2)', color: 'var(--primary)', borderRadius: '1rem' }}>
              Pending
            </span>
          </div>
          <p style={{ fontSize: '0.9rem', color: 'var(--text-muted)', marginBottom: '1.5rem' }}>
            No outputs detected. Start by preparing your deployment variables or paste existing outputs.
          </p>
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <Link href="/wizard" className="btn btn-primary" style={{ fontSize: '0.85rem' }}>
              Launch Wizard
            </Link>
            <Link href="/outputs" className="btn btn-outline" style={{ fontSize: '0.85rem' }}>
              Input Outputs
            </Link>
          </div>
        </div>

        <div className="card">
          <h3 style={{ fontSize: '1.1rem', fontWeight: '600', marginBottom: '1rem' }}>Security Health</h3>
          <div style={{ display: 'flex', alignItems: 'center', gap: '1rem', marginBottom: '1.5rem' }}>
            <div style={{ fontSize: '2.5rem' }}>🛡️</div>
            <div>
              <p style={{ fontSize: '1.25rem', fontWeight: 'bold', lineHeight: 1 }}>Not Checked</p>
              <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>Run security check on your config</p>
            </div>
          </div>
          <Link href="/security" className="btn btn-outline" style={{ fontSize: '0.85rem', width: '100%', textAlign: 'center' }}>
            Analyze Security
          </Link>
        </div>

        <div className="card">
          <h3 style={{ fontSize: '1.1rem', fontWeight: '600', marginBottom: '1rem' }}>Quick Actions</h3>
          <ul style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
            <li>
              <Link href="/guide" style={{ fontSize: '0.9rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <span>➡️</span> VPN Connection Guide
              </Link>
            </li>
            <li>
              <Link href="/runbook" style={{ fontSize: '0.9rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <span>➡️</span> Troubleshooting Runbook
              </Link>
            </li>
          </ul>
        </div>
      </div>

      <section className="card" style={{ background: 'var(--accent)' }}>
        <h3 style={{ fontSize: '1rem', fontWeight: '600', marginBottom: '1rem', color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
          Recent Activity
        </h3>
        <div style={{ textAlign: 'center', padding: '2rem', border: '1px dashed var(--border)', borderRadius: '0.5rem' }}>
          <p style={{ fontSize: '0.9rem', color: 'var(--text-muted)' }}>No recent activity found. Changes will appear here as you interact with the console.</p>
        </div>
      </section>
    </div>
  );
}
