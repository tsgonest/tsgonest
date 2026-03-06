const defaultUrl = 'https://tsgonest.dev';

function normalizeUrl(url: string): string {
  return url.replace(/\/$/, '');
}

export const siteConfig = {
  name: 'tsgonest docs',
  title: 'tsgonest documentation',
  description:
    'Native-speed TypeScript compilation with generated validation, serialization, and OpenAPI 3.2 for NestJS.',
  url: normalizeUrl(process.env.NEXT_PUBLIC_SITE_URL || defaultUrl),
  locale: 'en_US',
};

export function absoluteUrl(path: string): string {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`;
  return `${siteConfig.url}${normalizedPath}`;
}
