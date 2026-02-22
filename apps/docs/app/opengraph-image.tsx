import { ImageResponse } from 'next/og';

export const alt = 'tsgonest documentation';
export const size = {
  width: 1200,
  height: 630,
};
export const contentType = 'image/png';
export const dynamic = 'force-static';
export const revalidate = false;

export default function OpenGraphImage() {
  return new ImageResponse(
    (
      <div
        style={{
          width: '100%',
          height: '100%',
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'space-between',
          background: 'linear-gradient(140deg, #020617 0%, #0f172a 55%, #1e293b 100%)',
          color: '#E2E8F0',
          padding: '56px',
          fontFamily: 'Inter, sans-serif',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '18px' }}>
          <div
            style={{
              width: '64px',
              height: '64px',
              borderRadius: '18px',
              background: 'linear-gradient(140deg, #22D3EE, #1D4ED8)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: '36px',
              fontWeight: 700,
              color: '#fff',
            }}
          >
            T
          </div>
          <div style={{ fontSize: 42, fontWeight: 700, letterSpacing: -1 }}>tsgonest docs</div>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
          <div style={{ fontSize: 62, fontWeight: 700, letterSpacing: -1.5, lineHeight: 1.08 }}>
            Native-speed TypeScript tooling for NestJS.
          </div>
          <div style={{ fontSize: 30, color: '#94A3B8', lineHeight: 1.35 }}>
            Compile with tsgo, generate runtime validators/serializers, and emit OpenAPI 3.1 from static analysis.
          </div>
        </div>
      </div>
    ),
    {
      ...size,
    },
  );
}
