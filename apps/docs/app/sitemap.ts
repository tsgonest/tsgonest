import type { MetadataRoute } from 'next';
import { absoluteUrl } from '@/lib/site';
import { source } from '@/lib/source';

export const dynamic = 'force-static';
export const revalidate = false;

export default function sitemap(): MetadataRoute.Sitemap {
  const staticRoutes = ['/', '/docs'];
  const docsRoutes = source.getPages().map((page) => page.url);
  const routes = [...new Set([...staticRoutes, ...docsRoutes])];
  const lastModified = new Date();

  return routes.map((route) => ({
    url: absoluteUrl(route),
    lastModified,
    changeFrequency: route === '/' ? 'weekly' : 'monthly',
    priority: route === '/' ? 1 : route === '/docs' ? 0.9 : 0.7,
  }));
}
