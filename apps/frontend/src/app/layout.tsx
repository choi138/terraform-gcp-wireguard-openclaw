import type { Metadata } from "next";
import "./globals.css";
import Sidebar from "@/components/Sidebar";

export const metadata: Metadata = {
  title: "Ops Console | WireGuard + OpenClaw",
  description: "Terraform-based Operation Console",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body>
        <div style={{ display: 'flex' }}>
          <Sidebar />
          <main style={{ 
            flex: 1, 
            marginLeft: '260px', 
            minHeight: '100vh',
            background: 'var(--background)' 
          }}>
            {children}
          </main>
        </div>
      </body>
    </html>
  );
}
