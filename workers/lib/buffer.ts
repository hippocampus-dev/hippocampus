import { DurableObject } from "cloudflare:workers";

export class Buffer extends DurableObject {
  private data: unknown[] = [];
  private resolved = false;
  private resolver: (() => void) | null = null;

  async write(items: unknown[]): Promise<void> {
    this.data.push(...items);
    this.resolved = true;
    this.resolver?.();
  }

  async read(timeoutMs: number): Promise<unknown[]> {
    if (!this.resolved) {
      await Promise.race([
        new Promise<void>((resolve) => {
          this.resolver = resolve;
        }),
        new Promise<void>((resolve) => setTimeout(resolve, timeoutMs)),
      ]);
    }

    const result = [...this.data];
    this.data = [];
    return result;
  }
}
