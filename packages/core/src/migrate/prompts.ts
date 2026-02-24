/**
 * Interactive prompts using Node.js built-in readline.
 * No external dependencies — bundled cleanly into migrate.cjs.
 */

import { createInterface } from "readline";

const rl = createInterface({ input: process.stdin, output: process.stdout });

function ask(question: string): Promise<string> {
  return new Promise((resolve) => {
    rl.question(question, (answer) => resolve(answer.trim()));
  });
}

/**
 * Ask a yes/no question. Returns true for yes (default when pressing Enter).
 */
export async function confirm(message: string, defaultYes = true): Promise<boolean> {
  const hint = defaultYes ? "[Y/n]" : "[y/N]";
  const answer = await ask(`  ${message} ${hint} `);
  if (answer === "") return defaultYes;
  return answer.toLowerCase().startsWith("y");
}

/**
 * Close the readline interface. Must be called when prompts are done
 * or the process will hang.
 */
export function closePrompts(): void {
  rl.close();
}
