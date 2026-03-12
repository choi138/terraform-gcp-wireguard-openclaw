'use client';

import { useState, useEffect } from 'react';

type SecurityIssue = {
  id: string;
  severity: 'critical' | 'high' | 'medium' | 'info';
  title: string;
  description: string;
  fixHint: string;
};

export default function SecurityPage() {
  const [issues, setIssues] = useState<SecurityIssue[]>([]);
  const [loading, setLoading] = useState(false);

  const runCheck = () => {
    setLoading(true);
    
    // Simulate API call/analysis
    setTimeout(() => {
      const data = localStorage.getItem('ops-console-input');
      const input = data ? JSON.parse(data) : {};
      
      const newIssues: SecurityIssue[] = [];
      
      if (input.ui_source_ranges === '0.0.0.0/0') {
        newIssues.push({
          id: '1',
          severity: 'high',
          title: 'Wide Open UI Access',
          description: 'WireGuard UI is accessible from any IP address (0.0.0.0/0).',
          fixHint: 'Restrict ui_source_ranges to your specific IP or CIDR.'
        });
      }
      
      if (input.ssh_source_ranges === '0.0.0.0/0') {
        newIssues.push({
          id: '2',
          severity: 'high',
          title: 'SSH Exposed to World',
          description: 'SSH port is open to all IP addresses.',
          fixHint: 'Restrict ssh_source_ranges to a trusted management network.'
        });
      }
      
      if (input.openclaw_enable_public_ip) {
        newIssues.push({
          id: '3',
          severity: 'critical',
          title: 'Public IP Enabled for OpenClaw',
          description: 'OpenClaw instance has a public IP assigned, bypassing VPN requirement.',
          fixHint: 'Set openclaw_enable_public_ip to false and use WireGuard for access.'
        });
      }
      
      if (input.wg_port === 51820) {
        newIssues.push({
          id: '4',
          severity: 'info',
          title: 'Default WireGuard Port',
          description: 'Using the default UDP port 51820.',
          fixHint: 'Consider changing wg_port to an obscure number to avoid simple scans.'
        });
      }

      setIssues(newIssues);
      setLoading(false);
    }, 800);
  };

  const getSeverityColor = (severity: string) => {
    switch (severity) {
      case 'critical': return 'var(--severity-critical)';
      case 'high': return 'var(--severity-high)';
      case 'medium': return 'var(--severity-medium)';
      default: return 'var(--severity-info)';
    }
  };

  return (
    <div className="container">
      <header style={{ marginBottom: '2rem', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <h2 style={{ fontSize: '1.75rem', fontWeight: 'bold' }}>Security Check</h2>
          <p style={{ color: 'var(--text-muted)' }}>Analyze your configuration for security risks.</p>
        </div>
        <button onClick={runCheck} className="btn btn-primary" disabled={loading}>
          {loading ? 'Analyzing...' : 'Run Analysis'}
        </button>
      </header>

      {issues.length === 0 && !loading ? (
        <div className="card" style={{ textAlign: 'center', padding: '4rem' }}>
          <div style={{ fontSize: '3rem', marginBottom: '1rem' }}>✅</div>
          <h3 style={{ fontSize: '1.25rem', marginBottom: '0.5rem' }}>No Issues Detected</h3>
          <p style={{ color: 'var(--text-muted)' }}>Run the analysis to verify your configuration.</p>
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
          {issues.map(issue => (
            <div key={issue.id} className="card" style={{ 
              borderLeft: `4px solid ${getSeverityColor(issue.severity)}`,
              display: 'flex',
              gap: '1.5rem',
              alignItems: 'flex-start'
            }}>
              <div style={{ 
                background: getSeverityColor(issue.severity),
                color: 'white',
                padding: '0.25rem 0.5rem',
                borderRadius: '0.25rem',
                fontSize: '0.7rem',
                fontWeight: 'bold',
                textTransform: 'uppercase'
              }}>
                {issue.severity}
              </div>
              <div style={{ flex: 1 }}>
                <h3 style={{ fontSize: '1.1rem', fontWeight: '600', marginBottom: '0.5rem' }}>{issue.title}</h3>
                <p style={{ fontSize: '0.9rem', color: 'var(--foreground)', marginBottom: '0.75rem' }}>{issue.description}</p>
                <div style={{ 
                  background: 'rgba(255, 255, 255, 0.05)', 
                  padding: '0.75rem', 
                  borderRadius: '0.375rem',
                  fontSize: '0.85rem'
                }}>
                  <strong style={{ color: 'var(--primary)' }}>How to fix:</strong> {issue.fixHint}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
