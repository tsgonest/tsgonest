import remarkDirective from 'remark-directive';
import { remarkDirectiveAdmonition, remarkMdxFiles } from 'fumadocs-core/mdx-plugins';
import { defineConfig, defineDocs } from 'fumadocs-mdx/config';
import { metaSchema, pageSchema } from 'fumadocs-core/source/schema';
import convert from 'npm-to-yarn';

export const docs = defineDocs({
  dir: 'content/docs',
  docs: {
    schema: pageSchema,
    postprocess: {
      includeProcessedMarkdown: true,
    },
  },
  meta: {
    schema: metaSchema,
  },
});

export default defineConfig({
  mdxOptions: {
    remarkPlugins: [remarkDirective, remarkDirectiveAdmonition, remarkMdxFiles],
    remarkCodeTabOptions: {
      parseMdx: true,
    },
    // Use the same Shiki themes Prisma docs uses for visual parity
    rehypeCodeOptions: {
      themes: {
        light: 'github-light',
        dark: 'github-dark',
      },
    },
    remarkNpmOptions: {
      persist: {
        id: 'package-manager',
      },
      packageManagers: [
        { command: (cmd: string) => convert(cmd, 'npm'), name: 'npm' },
        { command: (cmd: string) => convert(cmd, 'pnpm'), name: 'pnpm' },
        { command: (cmd: string) => convert(cmd, 'yarn'), name: 'yarn' },
        {
          command: (cmd: string) => {
            const converted = convert(cmd, 'bun');
            if (!converted) return undefined;
            return converted.replace(/^bun x /, 'bunx --bun ');
          },
          name: 'bun',
        },
      ],
    },
  },
});
