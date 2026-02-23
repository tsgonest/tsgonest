import type { BaseLayoutProps, LinkItemType } from 'fumadocs-ui/layouts/shared';
import type { SidebarTabWithProps } from 'fumadocs-ui/components/sidebar/tabs/dropdown';
import Link from 'next/link';
import Image from 'next/image';

export const gitConfig = {
  user: 'tsgonest',
  repo: 'tsgonest',
  branch: 'main',
};

export const docsTabs: SidebarTabWithProps[] = [
  {
    title: 'Getting Started',
    url: '/docs',
    urls: new Set(['/docs', '/docs/getting-started']),
  },
  {
    title: 'CLI',
    url: '/docs/cli',
    urls: new Set(['/docs/cli']),
  },
  {
    title: 'Config',
    url: '/docs/config',
    urls: new Set(['/docs/config']),
  },
  {
    title: 'Validation',
    url: '/docs/validation',
    urls: new Set([
      '/docs/validation',
      '/docs/validation/jsdoc',
      '/docs/validation/string-tags',
      '/docs/validation/numeric-tags',
      '/docs/validation/array-tags',
      '/docs/validation/transforms',
      '/docs/validation/custom',
    ]),
  },
  {
    title: 'Serialization',
    url: '/docs/serialization-runtime',
    urls: new Set(['/docs/serialization-runtime']),
  },
  {
    title: 'OpenAPI',
    url: '/docs/openapi',
    urls: new Set([
      '/docs/openapi',
      '/docs/openapi/controllers',
      '/docs/openapi/parameters',
      '/docs/openapi/returns',
      '/docs/openapi/versioning',
      '/docs/openapi/config',
      '/docs/openapi/migration',
    ]),
  },
  {
    title: 'Comparisons',
    url: '/docs/comparisons/vs-nestjs-cli',
    urls: new Set([
      '/docs/comparisons/vs-nestjs-cli',
      '/docs/comparisons/vs-nestia-typia',
      '/docs/comparisons/vs-tsgo',
    ]),
  },
];

/** GitHub icon — same SVG Prisma uses in their navbar */
const GitHubIcon = (
  <svg role="img" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
    <path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12" />
  </svg>
);

export const links: LinkItemType[] = [
  {
    type: 'icon',
    label: 'GitHub',
    text: 'GitHub',
    url: `https://github.com/${gitConfig.user}/${gitConfig.repo}`,
    icon: GitHubIcon,
    external: true,
  },
];

export function baseOptions(): BaseLayoutProps {
  return {
    nav: {
      title: (
        <Link href="/docs" className="group inline-flex items-center gap-2">
          <Image
            src="/logo-mark.svg"
            alt="tsgonest"
            width={26}
            height={26}
            className="size-[26px] rounded-md shrink-0"
            priority
          />
          <span className="font-semibold tracking-tight text-fd-foreground">
            tsgonest
          </span>
          <span
            className="text-fd-muted-foreground select-none"
            aria-hidden="true"
          >
            /
          </span>
          <span className="font-mono text-base font-medium tracking-tight text-fd-muted-foreground group-hover:text-fd-foreground transition-colors">
            docs
          </span>
        </Link>
      ),
      transparentMode: 'none',
    },
    links,
  };
}
