import Link from 'next/link';

const navItems = [
  { href: '/', label: 'Home', icon: '🏠' },
  { href: '/wizard', label: 'Deploy Wizard', icon: '🧙' },
  { href: '/security', label: 'Security Check', icon: '🛡️' },
  { href: '/outputs', label: 'Outputs Dashboard', icon: '📊' },
  { href: '/guide', label: 'Connect Guide', icon: '🗺️' },
  { href: '/runbook', label: 'Runbook', icon: '📖' },
];

export default function Sidebar() {
  return (
    <aside style={{
      width: '260px',
      background: 'var(--accent)',
      borderRight: '1px solid var(--border)',
      height: '100vh',
      position: 'fixed',
      left: 0,
      top: 0,
      display: 'flex',
      flexDirection: 'column',
      padding: '1.5rem 1rem'
    }}>
      <div style={{ marginBottom: '2rem', padding: '0 0.5rem' }}>
        <h1 style={{ fontSize: '1.25rem', fontWeight: 'bold', color: 'var(--primary)' }}>
          Ops Console
        </h1>
        <p style={{ fontSize: '0.75rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
          OpenClaw + WireGuard
        </p>
      </div>
      
      <nav>
        <ul style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
          {navItems.map((item) => (
            <li key={item.href}>
              <Link href={item.href} style={{
                display: 'flex',
                alignItems: 'center',
                gap: '0.75rem',
                padding: '0.75rem 1rem',
                borderRadius: '0.375rem',
                fontSize: '0.9rem',
                transition: 'background 0.2s',
              }}>
                <span>{item.icon}</span>
                {item.label}
              </Link>
            </li>
          ))}
        </ul>
      </nav>
      
      <div style={{ marginTop: 'auto', padding: '1rem 0.5rem' }}>
        <div style={{ 
          background: 'rgba(13, 148, 136, 0.1)', 
          padding: '0.75rem', 
          borderRadius: '0.5rem',
          border: '1px solid rgba(13, 148, 136, 0.2)'
        }}>
          <p style={{ fontSize: '0.7rem', color: 'var(--text-muted)' }}>PROJECT STATUS</p>
          <p style={{ fontSize: '0.85rem', fontWeight: '600', color: 'var(--primary)' }}>Ready for Deploy</p>
        </div>
      </div>
    </aside>
  );
}
