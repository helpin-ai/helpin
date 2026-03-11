// src/persistence/memory.ts

export class MemoryPersistence {
  private storage: Record<string, any> = {};

  set(key: string, value: any): void {
    this.storage[key] = value;
  }

  get(key: string): any {
    return this.storage[key];
  }

  remove(key: string): void {
    delete this.storage[key];
  }

  save(): void {
    // No-op for memory persistence
  }

  clear(): void {
    this.storage = {};
  }
}
