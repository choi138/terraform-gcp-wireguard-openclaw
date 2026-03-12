'use client';

import { useState } from 'react';

type TerraformOutputs = {
  vpn_public_ip?: { value: string };
  wgeasy_ui_url?: { value: string };
  openclaw_gateway_url?: { value: string };
  vpn_internal_ip?: { value: string };
  openclaw_internal_ip?: { value: string };
};

export default function OutputsPage() {
  const [jsonInput, setJsonInput] = useState('');
  const [outputs, setOutputs] = useState<TerraformOutputs | null>(null);
  const [error, setError] = useState('');

  const handleParse = () => {
    try {
      const parsed = JSON.parse(jsonInput);
      setOutputs(parsed);
      setError('');
    } catch (e) {
      setError('Invalid JSON format. Please paste the exact output of "terraform output -json".');
      setOutputs(null);
    }
  };

  const copyValue = (val?: string) => {
    if (val) {
      navigator.clipboard.writeText(val);
      alert('Copied to clipboard!');
    }
  };

  const OutputCard = ({ title, value, isUrl = false }: { title: string, value?: string, isUrl?: boolean }) => {
    if (!value) return null;
    return (
      <div className="card" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <h4 style={{ fontSize: '0.75rem', color: 'var(--text-muted)', textTransform: 'uppercase', marginBottom: '0.25rem' }}>{title}</h4>
          <p style={{ fontSize: '1.1rem', fontWeight: 'bold', fontFamily: 'var(--font-mono)', color: 'var(--primary)' }}>{value}</p>
        </div>
        <div style={{ display: 'flex', gap: '0.5rem' }}>
          <button onClick={() => copyValue(value)} className="btn btn-outline" style={{ fontSize: '0.75rem' }}>Copy</button>
          {isUrl && (
            <a href={value} target="_blank" rel="noopener noreferrer" className="btn btn-primary" style={{ fontSize: '0.75rem' }}>
              Open URL
            </a>
          )}
        </div>
      </div>
    );
  };

  return (
    <div className="container">
      <header style={{ marginBottom: '2rem' }}>
        <h2 style={{ fontSize: '1.75rem', fontWeight: 'bold' }}>Outputs Dashboard</h2>
        <p style={{ color: 'var(--text-muted)' }}>Paste your Terraform output to visualize access points.</p>
      </header>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '2rem' }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
          <h3 style={{ fontSize: '1rem', fontWeight: '600' }}>JSON Input</h3>
          <textarea 
            style={{ width: '100%', minHeight: '300px', fontFamily: 'var(--font-mono)', fontSize: '0.85rem' }}
            placeholder='Paste result of "terraform output -json" here...'
            value={jsonInput}
            onChange={(e) => setJsonInput(e.target.value)}
          />
          {error && <p style={{ color: 'var(--severity-critical)', fontSize: '0.85rem' }}>{error}</p>}
          <button onClick={handleParse} className="btn btn-primary">Parse Outputs</button>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
          <h3 style={{ fontSize: '1rem', fontWeight: '600' }}>Parsed Values</h3>
          {!outputs ? (
            <div className="card" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '300px', borderStyle: 'dashed' }}>
              <p style={{ color: 'var(--text-muted)', textAlign: 'center' }}>No data parsed yet. <br/>Paste JSON and click "Parse Outputs".</p>
            </div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
              <OutputCard title="VPN Public IP" value={outputs.vpn_public_ip?.value} />
              <OutputCard title="WG-Easy UI URL" value={outputs.wgeasy_ui_url?.value} isUrl />
              <OutputCard title="OpenClaw Gateway URL" value={outputs.openclaw_gateway_url?.value} isUrl />
              <OutputCard title="VPN Internal IP" value={outputs.vpn_internal_ip?.value} />
              <OutputCard title="OpenClaw Internal IP" value={outputs.openclaw_internal_ip?.value} />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
