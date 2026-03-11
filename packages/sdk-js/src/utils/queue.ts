import { Transport } from '../core/types';
import { getLogger, Logger } from '../utils/logger';
import { LocalStoragePersistence } from '../persistence/local-storage';
import { isWindowAvailable } from '../utils/common';

export class RetryQueue {
  private queue: QueueItem[] = [];
  private processing: boolean = false;
  private batchTimeoutId: number | null = null;
  private persistence: LocalStoragePersistence;
  private isOnline: boolean = true; // Default to true for server-side
  private maxQueueItems: number;
  private maxQueueBytes: number;
  private totalQueueBytes: number = 0;
  constructor(
    private transport: Transport,
    private maxRetries: number = 3,
    private retryInterval: number = 1000,
    private batchSize: number = 10,
    private batchInterval: number = 1000,
    private logger: Logger = getLogger(),
    namespace: string = 'default',
    maxQueueItems: number = 1000,
    maxQueueBytes: number = 2_500_000,
  ) {
    this.persistence = new LocalStoragePersistence(
      `offline_queue_${namespace}`,
    );
    this.maxQueueItems = maxQueueItems;
    this.maxQueueBytes = maxQueueBytes;
    if (isWindowAvailable()) {
      this.isOnline = navigator.onLine;
      this.loadQueueFromStorage();
      this.initNetworkListeners();
      this.scheduleBatch();
    }
  }

  add(payload: any): void {
    const item = {
      payload,
      retries: 0,
      timestamp: Date.now(),
    } as QueueItem;
    item.bytes = this.estimateItemBytes(item);
    this.queue.push(item);
    this.totalQueueBytes += item.bytes;
    this.enforceQueueLimits();
    if (isWindowAvailable()) {
      this.saveQueueToStorage();
    } else {
      this.processBatch(); // Immediately process on server-side
    }
  }

  private initNetworkListeners(): void {
    if (isWindowAvailable()) {
      window.addEventListener('online', () => {
        this.isOnline = true;
        this.processBatch();
      });
      window.addEventListener('offline', () => {
        this.isOnline = false;
      });
    }
  }

  private scheduleBatch(): void {
    if (!isWindowAvailable()) return;

    if (this.batchTimeoutId !== null) {
      clearTimeout(this.batchTimeoutId);
    }

    this.batchTimeoutId = window.setTimeout(
      () => this.processBatch(),
      this.batchInterval,
    );
  }

  private async processBatch(): Promise<void> {
    if (
      (!isWindowAvailable() || this.isOnline) &&
      !this.processing &&
      this.queue.length > 0
    ) {
      this.processing = true;
      const batch = this.queue.splice(0, this.batchSize);
      this.totalQueueBytes = Math.max(
        0,
        this.totalQueueBytes -
          batch.reduce((sum, item) => sum + (item.bytes || 0), 0),
      );
      const payloads = batch.map((item) => item.payload);

      try {
        await this.transport.send(payloads);
        this.logger.debug(
          `Successfully sent batch of ${batch.length} payloads`,
        );
        if (isWindowAvailable()) {
          this.saveQueueToStorage();
        }
      } catch (error) {
        this.logger.error('Failed to send batch', error);
        await this.handleBatchFailure(batch);
      }

      this.processing = false;
    }

    if (isWindowAvailable()) {
      this.scheduleBatch();
    }
  }

  private async handleBatchFailure(batch: QueueItem[]): Promise<void> {
    for (const item of batch) {
      if (item.retries < this.maxRetries) {
        item.retries++;
        this.queue.unshift(item);
        this.totalQueueBytes += item.bytes || this.estimateItemBytes(item);
        this.enforceQueueLimits();
        this.logger.warn(`Retry attempt ${item.retries} for payload`);
      } else {
        this.logger.error(
          'Max retries reached, discarding payload',
          item.payload,
        );
      }
    }

    if (isWindowAvailable()) {
      this.saveQueueToStorage();
      await new Promise((resolve) => setTimeout(resolve, this.retryInterval));
    }
  }

  private enforceQueueLimits(): void {
    if (this.maxQueueItems > 0 && this.queue.length > this.maxQueueItems) {
      const excess = this.queue.length - this.maxQueueItems;
      const dropped = this.queue.splice(0, excess);
      this.totalQueueBytes = Math.max(
        0,
        this.totalQueueBytes -
          dropped.reduce((sum, item) => sum + (item.bytes || 0), 0),
      );
      this.logger.warn(
        `Retry queue exceeded ${this.maxQueueItems} items; dropped ${dropped.length} oldest payload(s)`,
      );
    }

    if (
      this.maxQueueBytes > 0 &&
      this.totalQueueBytes > this.maxQueueBytes &&
      this.queue.length > 0
    ) {
      let droppedCount = 0;
      while (
        this.totalQueueBytes > this.maxQueueBytes &&
        this.queue.length > 1 // Keep at least 1 item
      ) {
        const removed = this.queue.shift();
        if (removed) {
          this.totalQueueBytes -= removed.bytes || this.estimateItemBytes(removed);
          droppedCount++;
        }
      }
      this.totalQueueBytes = Math.max(0, this.totalQueueBytes);
      if (droppedCount > 0) {
        this.logger.warn(
          `Retry queue exceeded ${this.maxQueueBytes} bytes; dropped ${droppedCount} oldest payload(s)`,
        );
      }
    }
  }

  private estimateItemBytes(item: QueueItem): number {
    const serializable = { ...item };
    delete (serializable as any).bytes;
    try {
      return JSON.stringify(serializable).length;
    } catch (_e) {
      return 0;
    }
  }

  private loadQueueFromStorage(): void {
    if (isWindowAvailable()) {
      const storedQueue = this.persistence.get('queue');
      if (!storedQueue) return;

      try {
        const parsedQueue: QueueItem[] = JSON.parse(storedQueue);
        this.queue = parsedQueue.map((item) => ({
          ...item,
          bytes: item.bytes || this.estimateItemBytes(item),
        }));
        this.totalQueueBytes = this.queue.reduce(
          (sum, item) => sum + (item.bytes || 0),
          0,
        );
        this.enforceQueueLimits();
      } catch (error) {
        this.logger.error('Failed to parse stored queue', error);
        this.queue = [];
        this.totalQueueBytes = 0;
      }
    }
  }

  private saveQueueToStorage(): void {
    if (isWindowAvailable()) {
      const serializableQueue = this.queue.map(({ bytes, ...rest }) => rest);
      this.persistence.set('queue', JSON.stringify(serializableQueue));
    }
  }
}

interface QueueItem {
  payload: any;
  retries: number;
  timestamp: number;
  bytes?: number;
}
