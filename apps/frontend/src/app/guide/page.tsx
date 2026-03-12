'use client';

import { useState, useEffect } from 'react';

type Step = {
  id: string;
  title: string;
  description: string;
  checked: boolean;
};

const defaultSteps: Step[] = [
  { id: '1', title: 'Access WG-Easy UI', description: 'Open the wgeasy_ui_url in your browser.', checked: false },
  { id: '2', title: 'Create Client Profile', description: 'Enter a name (e.g., "my-laptop") and create a new client.', checked: false },
  { id: '3', title: 'Download Config/Scan QR', description: 'Download the .conf file or scan the QR code with your WireGuard app.', checked: false },
  { id: '4', title: 'Enable VPN Connection', description: 'Activate the tunnel in your WireGuard client.', checked: false },
  { id: '5', title: 'Verify Gateway Access', description: 'Try accessing openclaw_gateway_url while VPN is active.', checked: false },
];

export default function GuidePage() {
  const [steps, setSteps] = useState<Step[]>(defaultSteps);

  useEffect(() => {
    const saved = localStorage.getItem('ops-console-guide-steps');
    if (saved) {
      setSteps(JSON.parse(saved));
    }
  }, []);

  const toggleStep = (id: string) => {
    const newSteps = steps.map(s => s.id === id ? { ...s, checked: !s.checked } : s);
    setSteps(newSteps);
    localStorage.setItem('ops-console-guide-steps', JSON.stringify(newSteps));
  };

  const progress = Math.round((steps.filter(s => s.checked).length / steps.length) * 100);

  return (
    <div className="container">
      <header style={{ marginBottom: '2rem' }}>
        <h2 style={{ fontSize: '1.75rem', fontWeight: 'bold' }}>Connect Guide</h2>
        <p style={{ color: 'var(--text-muted)' }}>Follow these steps to establish a secure connection.</p>
      </header>

      <div className="card" style={{ marginBottom: '2rem', padding: '1rem', background: 'var(--accent)' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '0.5rem' }}>
          <span style={{ fontSize: '0.85rem', fontWeight: 'bold' }}>Setup Progress</span>
          <span style={{ fontSize: '0.85rem', color: 'var(--primary)' }}>{progress}%</span>
        </div>
        <div style={{ width: '100%', height: '8px', background: 'var(--border)', borderRadius: '4px' }}>
          <div style={{ 
            width: `${progress}%`, 
            height: '100%', 
            background: 'var(--primary)', 
            borderRadius: '4px',
            transition: 'width 0.3s ease'
          }} />
        </div>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
        {steps.map((step, index) => (
          <div 
            key={step.id} 
            className="card" 
            onClick={() => toggleStep(step.id)}
            style={{ 
              display: 'flex', 
              gap: '1.5rem', 
              alignItems: 'center', 
              cursor: 'pointer',
              opacity: step.checked ? 0.6 : 1,
              transition: 'opacity 0.2s'
            }}
          >
            <div style={{ 
              width: '2.5rem', 
              height: '2.5rem', 
              borderRadius: '50%', 
              background: step.checked ? 'var(--primary)' : 'var(--accent)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: '1.2rem',
              border: '2px solid var(--border)'
            }}>
              {step.checked ? '✓' : index + 1}
            </div>
            <div style={{ flex: 1 }}>
              <h3 style={{ fontSize: '1.1rem', fontWeight: '600', textDecoration: step.checked ? 'line-through' : 'none' }}>
                {step.title}
              </h3>
              <p style={{ fontSize: '0.9rem', color: 'var(--text-muted)' }}>{step.description}</p>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
