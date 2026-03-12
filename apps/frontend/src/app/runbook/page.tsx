'use client';

import { useState } from 'react';

type Scenario = {
  id: string;
  title: string;
  icon: string;
  commands: { label: string, cmd: string, explanation: string }[];
};

const scenarios: Scenario[] = [
  {
    id: 'vpn-access',
    title: 'VPN Connection Issues',
    icon: '🔌',
    commands: [
      { label: 'Check WG-Easy Logs', cmd: 'sudo docker logs wg-easy', explanation: 'View the logs of the WireGuard management container.' },
      { label: 'Check UDP Port', cmd: 'sudo netstat -ulnp | grep 51820', explanation: 'Verify if WireGuard is listening on the UDP port.' },
      { label: 'Verify Startup Script', cmd: 'sudo journalctl -u google-startup-scripts.service', explanation: 'Check if the initialization script ran successfully.' },
    ]
  },
  {
    id: 'gateway-access',
    title: 'Gateway Unreachable',
    icon: '🌐',
    commands: [
      { label: 'Check OpenClaw Status', cmd: 'sudo systemctl status openclaw', explanation: 'Check if the OpenClaw service is active.' },
      { label: 'Check Recent Logs', cmd: 'sudo journalctl -u openclaw -n 200', explanation: 'View the last 200 lines of OpenClaw logs.' },
      { label: 'Verify Internal Connectivity', cmd: 'ping 10.128.0.x', explanation: 'Test ping between VPN VM and Gateway VM.' },
    ]
  },
  {
    id: 'container-health',
    title: 'Container/VM Health',
    icon: '📦',
    commands: [
      { label: 'List All Containers', cmd: 'sudo docker ps -a', explanation: 'List all running and stopped containers.' },
      { label: 'Check VM Metrics', cmd: 'top -n 1', explanation: 'Quickly check CPU and Memory usage on the VM.' },
    ]
  }
];

export default function RunbookPage() {
  const [activeScenario, setActiveScenario] = useState<Scenario>(scenarios[0]);

  const copyCmd = (cmd: string) => {
    navigator.clipboard.writeText(cmd);
    alert('Command copied!');
  };

  return (
    <div className="container">
      <header style={{ marginBottom: '2rem' }}>
        <h2 style={{ fontSize: '1.75rem', fontWeight: 'bold' }}>Troubleshooting Runbook</h2>
        <p style={{ color: 'var(--text-muted)' }}>Quick commands and guides for common issues.</p>
      </header>

      <div style={{ display: 'grid', gridTemplateColumns: '260px 1fr', gap: '2rem' }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
          <h3 style={{ fontSize: '0.85rem', color: 'var(--text-muted)', textTransform: 'uppercase', marginBottom: '0.5rem' }}>Scenarios</h3>
          {scenarios.map(s => (
            <button 
              key={s.id} 
              onClick={() => setActiveScenario(s)}
              className="card"
              style={{ 
                textAlign: 'left', 
                padding: '0.75rem 1rem', 
                background: activeScenario.id === s.id ? 'var(--primary)' : 'var(--accent)',
                borderColor: activeScenario.id === s.id ? 'var(--primary)' : 'var(--border)',
                display: 'flex',
                alignItems: 'center',
                gap: '0.75rem'
              }}
            >
              <span style={{ fontSize: '1.2rem' }}>{s.icon}</span>
              <span style={{ fontSize: '0.9rem', fontWeight: activeScenario.id === s.id ? 'bold' : 'normal' }}>{s.title}</span>
            </button>
          ))}
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
          <h3 style={{ fontSize: '1.25rem', fontWeight: 'bold' }}>{activeScenario.title}</h3>
          {activeScenario.commands.map((c, i) => (
            <div key={i} className="card">
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.75rem' }}>
                <h4 style={{ fontWeight: '600' }}>{c.label}</h4>
                <button onClick={() => copyCmd(c.cmd)} className="btn btn-outline" style={{ fontSize: '0.75rem' }}>Copy Command</button>
              </div>
              <pre style={{ 
                background: '#000', 
                padding: '1rem', 
                borderRadius: '0.375rem', 
                fontFamily: 'var(--font-mono)', 
                fontSize: '0.9rem', 
                color: '#fff',
                marginBottom: '1rem',
                border: '1px solid var(--border)'
              }}>
                $ {c.cmd}
              </pre>
              <p style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>
                <strong style={{ color: 'var(--primary)' }}>Analysis:</strong> {c.explanation}
              </p>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
