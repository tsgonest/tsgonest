import { source } from '@/lib/source';
import { DocsLayout } from 'fumadocs-ui/layouts/notebook';
import { baseOptions, docsTabs } from '@/lib/layout.shared';

export default function Layout({ children }: LayoutProps<'/docs'>) {
  const options = baseOptions();

  return (
    <DocsLayout
      {...options}
      tree={source.getPageTree()}
      tabMode="navbar"
      nav={{ ...options.nav, mode: 'top', transparentMode: 'none' }}
      sidebar={{
        defaultOpenLevel: 1,
        collapsible: false,
        tabs: docsTabs,
        // Prisma uses a non-collapsible sidebar with a clean top-level tab strip
        banner: undefined,
        footer: undefined,
      }}
    >
      {children}
    </DocsLayout>
  );
}
