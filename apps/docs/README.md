# tsgonest docs

This is a standalone Next.js + Fumadocs app for the `tsgonest` project.

It is configured for static export (`next build` -> `out/`) and root-path hosting (`/`).

## SEO configuration

Set `NEXT_PUBLIC_SITE_URL` to your production docs URL so canonical links, sitemap, and social metadata are correct.

Example:

```bash
NEXT_PUBLIC_SITE_URL=https://docs.example.com pnpm build
```

## Commands

```bash
pnpm dev
```

Start the local dev server.

```bash
pnpm build
```

Build a static export into `out/`.

```bash
pnpm start
```

Serve the generated static output from `out/`.
