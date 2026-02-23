import Link from 'next/link';
import Image from 'next/image';
import type { Metadata } from 'next';
import { absoluteUrl, siteConfig } from '@/lib/site';

export const metadata: Metadata = {
  title: 'tsgonest documentation',
  description: siteConfig.description,
  alternates: { canonical: '/' },
  openGraph: {
    type: 'website',
    title: 'tsgonest documentation',
    description: siteConfig.description,
    url: absoluteUrl('/'),
    images: [absoluteUrl('/opengraph-image')],
  },
  twitter: {
    card: 'summary_large_image',
    title: 'tsgonest documentation',
    description: siteConfig.description,
    images: [absoluteUrl('/twitter-image')],
  },
};

/* -- Feature sections -------------------------------------------------- */

const coreFeatures = [
  {
    href: '/docs/getting-started',
    label: 'Getting Started',
    title: 'Get up and running fast',
    description:
      'Install tsgonest, configure your project, and build your first NestJS app with native-speed tsgo in minutes.',
    icon: (
      <svg
        xmlns="http://www.w3.org/2000/svg"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
        className="size-5"
      >
        <path d="M13 2 3 14h9l-1 8 10-12h-9l1-8z" />
      </svg>
    ),
  },
  {
    href: '/docs/cli',
    label: 'CLI Reference',
    title: 'Full CLI reference',
    description:
      'Every flag for `tsgonest build` and `tsgonest dev`, the operational pipeline, exit codes, and usage examples.',
    icon: (
      <svg
        xmlns="http://www.w3.org/2000/svg"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
        className="size-5"
      >
        <polyline points="4 17 10 11 4 5" />
        <line x1="12" y1="19" x2="20" y2="19" />
      </svg>
    ),
  },
  {
    href: '/docs/config',
    label: 'Config',
    title: 'Configuration reference',
    description:
      'Complete `tsgonest.config.json` schema with controllers, transforms, OpenAPI, NestJS versioning, and more.',
    icon: (
      <svg
        xmlns="http://www.w3.org/2000/svg"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
        className="size-5"
      >
        <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z" />
        <circle cx="12" cy="12" r="3" />
      </svg>
    ),
  },
];

const deepDiveFeatures = [
  {
    href: '/docs/validation',
    label: 'Validation',
    title: 'Validation & constraints',
    description:
      'JSDoc tags, branded types, string formats, transforms, coercion, custom validators, discriminated unions, and Standard Schema.',
    accent: 'text-teal-600 bg-teal-50 dark:bg-teal-950/40 dark:text-teal-400',
  },
  {
    href: '/docs/serialization-runtime',
    label: 'Serialization',
    title: 'Serialization & runtime',
    description:
      'Fast generated serializers (2-5x faster than JSON.stringify), manifest-driven discovery, ValidationPipe, and FastInterceptor.',
    accent: 'text-sky-600 bg-sky-50 dark:bg-sky-950/40 dark:text-sky-400',
  },
  {
    href: '/docs/openapi',
    label: 'OpenAPI',
    title: 'OpenAPI 3.2 generation',
    description:
      'Static analysis of NestJS controllers produces a fully-compliant OpenAPI 3.2 document with zero runtime decorators.',
    accent: 'text-violet-500 bg-violet-50 dark:bg-violet-950/40 dark:text-violet-400',
  },
];

const comparisonFeatures = [
  {
    href: '/docs/comparisons/vs-nestjs-cli',
    label: 'vs NestJS CLI',
    title: 'tsgonest vs NestJS CLI',
    description:
      'Build speed, validation, serialization, and OpenAPI compared to the standard NestJS ecosystem.',
    accent: 'text-rose-500 bg-rose-50 dark:bg-rose-950/40 dark:text-rose-400',
  },
  {
    href: '/docs/comparisons/vs-nestia-typia',
    label: 'vs Nestia + Typia',
    title: 'tsgonest vs Nestia + Typia',
    description:
      'Architecture differences, DX comparison, and migration path from the Nestia/Typia ecosystem.',
    accent: 'text-amber-600 bg-amber-50 dark:bg-amber-950/40 dark:text-amber-400',
  },
  {
    href: '/docs/comparisons/vs-tsgo',
    label: 'vs tsgo',
    title: 'tsgonest vs tsgo',
    description:
      'What tsgonest adds on top of Microsoft\'s typescript-go compiler: companions, manifest, OpenAPI, and dev mode.',
    accent: 'text-blue-600 bg-blue-50 dark:bg-blue-950/40 dark:text-blue-400',
  },
];

/* -- Page -------------------------------------------------------------- */

export default function HomePage() {
  return (
    <main className="mx-auto flex w-full max-w-[min(calc(var(--fd-layout-width,97rem)-268px),1000px)] flex-1 flex-col gap-16 px-4 py-14 md:px-8 md:py-16 xl:py-20">

      {/* -- Hero --------------------------------------------------------- */}
      <section className="flex flex-col gap-5">
        <div className="flex items-center gap-3">
          <Image
            src="/logo-mark.svg"
            alt="tsgonest logo"
            width={52}
            height={52}
            className="size-[52px] rounded-xl shrink-0"
            priority
          />
        </div>

        <div className="flex flex-col gap-3">
          <h1 className="text-4xl font-bold tracking-tight md:text-5xl">
            tsgonest documentation
          </h1>
          <p className="max-w-2xl text-balance text-lg leading-relaxed text-fd-muted-foreground">
            Native-speed TypeScript compilation with{' '}
            <code className="rounded-square border border-[var(--color-stroke-neutral)] bg-[var(--color-background-neutral-weak)] px-1.5 py-0.5 text-sm font-medium text-fd-foreground">
              tsgo
            </code>
            , generated runtime validators and serializers, and static OpenAPI 3.2
            analysis for NestJS.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-3 pt-1">
          <Link
            href="/docs/getting-started"
            className="inline-flex items-center gap-2 rounded-square bg-[var(--primary)] px-4 py-2 text-sm font-semibold text-white transition-opacity hover:opacity-90"
          >
            Get started
            <svg
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              className="size-3.5"
              aria-hidden="true"
            >
              <path d="M5 12h14M12 5l7 7-7 7" />
            </svg>
          </Link>
          <Link
            href="/docs"
            className="inline-flex items-center gap-2 rounded-square border border-[var(--color-stroke-neutral)] bg-[var(--color-background-default)] px-4 py-2 text-sm font-medium text-fd-foreground transition-colors hover:bg-[var(--color-background-neutral-weak)]"
          >
            Read the docs
          </Link>
        </div>
      </section>

      {/* -- Core feature cards --------------------------------------------- */}
      <section className="flex flex-col gap-4">
        <h2 className="text-xs font-semibold uppercase tracking-wider text-fd-muted-foreground">
          Explore the docs
        </h2>
        <div className="grid gap-3 sm:grid-cols-3">
          {coreFeatures.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className="group flex flex-col gap-3 rounded-square border border-[var(--color-stroke-neutral)] bg-[var(--color-background-default)] p-5 transition-colors hover:border-[var(--color-stroke-neutral-strong)] hover:bg-[var(--color-background-neutral-weak)]"
            >
              <div className="flex items-center gap-2 text-fd-muted-foreground group-hover:text-[var(--primary)] transition-colors">
                {item.icon}
                <span className="text-xs font-semibold uppercase tracking-wider">
                  {item.label}
                </span>
              </div>
              <div>
                <p className="mb-1 font-semibold tracking-tight text-fd-foreground">
                  {item.title}
                </p>
                <p className="text-sm leading-relaxed text-fd-muted-foreground">
                  {item.description}
                </p>
              </div>
            </Link>
          ))}
        </div>
      </section>

      {/* -- Deep-dive section ---------------------------------------------- */}
      <section className="flex flex-col gap-4">
        <h2 className="text-xs font-semibold uppercase tracking-wider text-fd-muted-foreground">
          Deep dive
        </h2>
        <div className="grid gap-3 sm:grid-cols-3">
          {deepDiveFeatures.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className="group flex flex-col gap-3 rounded-square border border-[var(--color-stroke-neutral)] bg-[var(--color-background-default)] p-5 transition-colors hover:border-[var(--color-stroke-neutral-strong)] hover:bg-[var(--color-background-neutral-weak)]"
            >
              <span
                className={`inline-flex w-fit items-center rounded-square px-2 py-0.5 text-xs font-semibold ${item.accent}`}
              >
                {item.label}
              </span>
              <div>
                <p className="mb-1 font-semibold tracking-tight text-fd-foreground">
                  {item.title}
                </p>
                <p className="text-sm leading-relaxed text-fd-muted-foreground">
                  {item.description}
                </p>
              </div>
            </Link>
          ))}
        </div>
      </section>

      {/* -- Comparisons section -------------------------------------------- */}
      <section className="flex flex-col gap-4">
        <h2 className="text-xs font-semibold uppercase tracking-wider text-fd-muted-foreground">
          Comparisons
        </h2>
        <div className="grid gap-3 sm:grid-cols-3">
          {comparisonFeatures.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className="group flex flex-col gap-3 rounded-square border border-[var(--color-stroke-neutral)] bg-[var(--color-background-default)] p-5 transition-colors hover:border-[var(--color-stroke-neutral-strong)] hover:bg-[var(--color-background-neutral-weak)]"
            >
              <span
                className={`inline-flex w-fit items-center rounded-square px-2 py-0.5 text-xs font-semibold ${item.accent}`}
              >
                {item.label}
              </span>
              <div>
                <p className="mb-1 font-semibold tracking-tight text-fd-foreground">
                  {item.title}
                </p>
                <p className="text-sm leading-relaxed text-fd-muted-foreground">
                  {item.description}
                </p>
              </div>
            </Link>
          ))}
        </div>
      </section>

      {/* -- Compilation pipeline callout ----------------------------------- */}
      <section className="rounded-square border border-[var(--color-stroke-neutral)] bg-[var(--color-background-neutral-weak)] p-6 md:p-8">
        <h2 className="mb-2 text-lg font-semibold tracking-tight">
          Compilation pipeline
        </h2>
        <p className="mb-6 text-sm leading-relaxed text-fd-muted-foreground">
          tsgonest wraps{' '}
          <a
            href="https://github.com/microsoft/typescript-go"
            rel="noreferrer noopener"
            target="_blank"
            className="font-medium text-[var(--primary)] hover:underline"
          >
            typescript-go
          </a>{' '}
          (tsgo) and runs a full static analysis pass before emitting companion files.
        </p>
        <ol className="flex flex-col gap-2 text-sm">
          {[
            'Parse CLI args + tsgonest.config.json',
            'Create tsgo program from tsconfig',
            'Type-check and emit JavaScript',
            'Walk AST with type checker \u2192 extract type metadata',
            'Generate *.tsgonest.js + *.tsgonest.d.ts companions',
            'Write __tsgonest_manifest.json',
            'Generate openapi.json from NestJS controllers',
          ].map((step, i) => (
            <li key={i} className="flex items-start gap-3">
              <span className="mt-px flex size-5 shrink-0 items-center justify-center rounded-full bg-[var(--primary)] text-[10px] font-bold text-white">
                {i + 1}
              </span>
              <span className="text-fd-foreground">{step}</span>
            </li>
          ))}
        </ol>
      </section>
    </main>
  );
}
